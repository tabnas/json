// Copyright (c) 2026 tabnas, MIT License

// Package json is a standard JSON grammar plugin for the tabnas parsing
// engine (github.com/tabnas/parser/go).
//
// The engine ships no grammar of its own; this package supplies the
// strict, standard-JSON one. The rule set (val / map / list / pair /
// elem) is jsonic's "Plain JSON" grammar — the pure-JSON core jsonic
// defines before extending it for the relaxed jsonic format. Here that
// core is installed on its own, with the lexer restricted to strict JSON
// and none of jsonic's extended grammar (comments, unquoted keys,
// implicit objects/arrays, trailing commas, single/backtick strings,
// path diving). It mirrors encoding/json: quoted-string keys only,
// double-quoted strings, plain decimal numbers, true/false/null.
//
// This plugin is intended to be the foundation other tabnas grammar
// plugins build on: Use it first, then layer rules on the shared val /
// map / list / pair / elem rules.
package tabnasjson

import (
	"math"
	"regexp"
	"strconv"
	"sync"

	tabnas "github.com/tabnas/parser/go"
)

// VERSION is this module's version. It MUST equal ts/package.json
// "version": the release orchestrator rewrites both, and
// TestVersionMatchesPackageJSON fails the build if they drift.
const VERSION = "0.5.3"

// JsonError is the error type returned by a failed parse — an alias of
// the engine's *tabnas.TabnasError (with Code / Row / Col / Hint fields
// and a formatted Error() report). Mirrors the TS re-export
// `export { TabnasError as JsonError }`; reach it with
// `errors.As(err, &je)` where `je` is a *tabnasjson.JsonError.
type JsonError = tabnas.TabnasError

// strictNumber matches exactly a standard JSON number. Anything the
// engine's (lenient) number matcher accepts that does not match this —
// leading `+`, a bare leading `.` (".5"), a trailing `.` ("1."), leading
// zeros ("01", "00") — is excluded and so rejected.
var strictNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// excludeNonStandardNumber rejects anything the engine's lenient number
// matcher accepts that standard JSON does not.
//
// Beyond the syntactic check, it also rejects out-of-range exponents
// ("1e999", "123123e100000"). Those are syntactically valid JSON, and the
// two platform oracles disagree on them: JS `JSON.parse` saturates to
// Infinity, while `encoding/json` fails with "cannot unmarshal number
// ... into Go value of type float64". This package is held to per-runtime
// parity (AGENTS.md: JSON.parse in TS/JS, encoding/json in Go), so the TS
// half accepts them and the Go half must not.
//
// The engine used to make this moot by dropping overflowing literals to
// the text matcher; it now saturates to ±Inf to match TS, so the
// rejection has to be stated here. Underflow ("1e-999" -> 0) is accepted
// by encoding/json and is left alone: ParseFloat reports no error for it.
func excludeNonStandardNumber(s string) bool {
	if !strictNumber.MatchString(s) {
		return true
	}
	f, err := strconv.ParseFloat(s, 64)
	return err != nil && math.IsInf(f, 0)
}

// jsonOptions restricts the engine to strict JSON. Mirrors JSON_OPTIONS
// in the TypeScript json.ts.
func jsonOptions() tabnas.Options {
	f := false
	tr := true
	return tabnas.Options{
		Text: &tabnas.TextOptions{Lex: &f},
		Number: &tabnas.NumberOptions{
			Hex: &f, Oct: &f, Bin: &f,
			Sep:     "",
			Exclude: excludeNonStandardNumber,
		},
		String: &tabnas.StringOptions{
			Chars:      `"`,
			MultiChars: "",
			// Standard JSON escape handling: AllowUnknown:false rejects any
			// unrecognized escape (\q, \z); EscapeStrict disables the
			// non-standard \xHH and \u{...} structural escapes (plain
			// \uXXXX stays); and dropping v / ' / ` from the escape map
			// removes the remaining non-standard built-ins. Result: exactly
			// the encoding/json escape set, identical to the TS engine.
			AllowUnknown: &f,
			EscapeStrict: &tr,
			Escape:       map[string]string{"v": "", "'": "", "`": ""},
		},
		Comment: &tabnas.CommentOptions{Lex: &f},
		Map:     &tabnas.MapOptions{Extend: &f},
		Lex:     &tabnas.LexOptions{Empty: &f},
		// Restrict the rule set to the `json`-tagged alternates (TS:
		// rule.include). The JSON grammar below tags every alt "json", so
		// on a bare engine this is inert; it matters when the options are
		// applied over an already-extended grammar, keeping only its
		// strict-JSON alternates.
		Rule: &tabnas.RuleOptions{Finish: &f, Include: "json"},
		// Treat a "no value" / NaN result as a parse failure, mirroring the
		// TS result.fail. Undefined is the engine's "no value" sentinel
		// (not nil — JSON null parses to nil and must stay valid); NaN
		// never matches via ==, included only for TS parity.
		Result: &tabnas.ResultOptions{Fail: []any{tabnas.Undefined, math.NaN()}},
		// Strict JSON keys are quoted strings only.
		TokenSet: map[string][]string{"KEY": {"#ST"}},
	}
}

// RegisterJSONGrammar installs the standard JSON rule set (val / map /
// list / pair / elem) on j via the engine's declarative grammar spec —
// the same shape as the TypeScript registerJsonGrammar. Exposed
// separately from the options so other grammar plugins can layer on the
// JSON core.
//
// The value tree is built ENTIRELY by the engine's native-value
// `$`-builtins (object/array/reset/key/setval/push/value); there are NO
// grammar-local closures. The builders are info-aware, so when the
// MapRef / ListRef / TextInfo options are enabled they allocate the
// engine's info carriers (MapRef / ListRef / Text) and perform the
// container/quote annotation themselves — the json plugin used to
// hand-write that as @map-bc / @list-bc / @val-bc state hooks. Strict
// JSON containers are always explicit, so @object$/@array$ take the
// default implicit:false (no `K` config needed).
//
// The builtin actions used below, one line each:
//
//	@reset$  — clear the parent-seeded node (so a value doesn't inherit the parent container).
//	@object$ — allocate an empty object into the node (a MapRef under info.Map).
//	@array$  — allocate an empty array into the node (a ListRef under info.List).
//	@key$    — capture the matched key token into a scratch slot for the pending @setval$.
//	@setval$ — assign the just-built child value into the object under the captured key.
//	@push$   — append the just-built child value to the array.
//	@value$  — resolve the rule's value: a built child wins, else the scalar token (a Text under info.Text).
func RegisterJSONGrammar(j *tabnas.Tabnas) error {
	rules := map[string]*tabnas.GrammarRuleSpec{
		// val: a value is a map, a list, or a plain scalar token. @reset$
		// clears the parent-seeded node so a scalar doesn't inherit the
		// parent container; @value$ coalesces (child wins, else the scalar
		// token) and boxes a string with its quote under TextInfo.
		"val": {
			Open: []*tabnas.GrammarAltSpec{
				{S: "#OB", P: "map", B: 1, A: "@reset$", G: "map,json"},
				{S: "#OS", P: "list", B: 1, A: "@reset$", G: "list,json"},
				{S: "#VAL", A: "@reset$", G: "val,json"},
			},
			Close: []*tabnas.GrammarAltSpec{
				{S: "#ZZ", A: "@value$", G: "end,json"},
				{B: 1, A: "@value$", G: "more,json"},
			},
		},
		// map: an object. @object$ allocates it (a MapRef under info.map).
		"map": {
			Open: []*tabnas.GrammarAltSpec{
				{S: "#OB #CB", B: 1, N: map[string]int{"pk": 0}, A: "@object$", G: "map,json"},
				{S: "#OB", P: "pair", N: map[string]int{"pk": 0}, A: "@object$", G: "map,json,pair"},
			},
			Close: []*tabnas.GrammarAltSpec{
				{S: "#CB", G: "end,json"},
			},
		},
		// list: an array. @array$ allocates it (a ListRef under info.list).
		"list": {
			Open: []*tabnas.GrammarAltSpec{
				{S: "#OS #CS", B: 1, A: "@array$", G: "list,json"},
				{S: "#OS", P: "elem", A: "@array$", G: "list,elem,json"},
			},
			Close: []*tabnas.GrammarAltSpec{
				{S: "#CS", G: "end,json"},
			},
		},
		// pair: a key:value entry inside a map. @key$ captures the key;
		// @setval$ assigns the built value under it.
		"pair": {
			Open: []*tabnas.GrammarAltSpec{
				{S: "#KEY #CL", P: "val", U: map[string]any{"pair": true}, A: "@key$", G: "map,pair,key,json"},
			},
			Close: []*tabnas.GrammarAltSpec{
				{S: "#CA", R: "pair", A: "@setval$", G: "map,pair,comma,json"},
				{S: "#CB", B: 1, A: "@setval$", G: "map,pair,close,json"},
			},
		},
		// elem: a value inside a list. @push$ appends the built value.
		"elem": {
			Open: []*tabnas.GrammarAltSpec{
				{P: "val", G: "list,elem,val,json"},
			},
			Close: []*tabnas.GrammarAltSpec{
				{S: "#CA", R: "elem", A: "@push$", G: "list,elem,comma,json"},
				{S: "#CS", B: 1, A: "@push$", G: "list,elem,close,json"},
			},
		},
	}

	// A Go map has no order, so without RuleOrder the engine falls back to
	// sorted names and (*Tabnas).RuleNames reports the grammar alphabetically
	// -- [elem list map pair val] -- where TypeScript reports it as declared:
	// [val map list pair elem]. Anything built on RuleNames inherits that,
	// railroad's extracted Go model among them.
	//
	// The names are written in the order the `rules` literal above declares
	// them, which is the order ts/src/json.ts declares them in.
	return j.Grammar(&tabnas.GrammarSpec{
		V:         2,
		Rule:      rules,
		RuleOrder: []string{"val", "map", "list", "pair", "elem"},
	})
}

// Json is the standard plugin form: apply the strict JSON options, then
// register the JSON grammar. Use it on a bare engine, or call Make.
func Json(j *tabnas.Tabnas, _ map[string]any) error {
	j.SetOptions(jsonOptions())
	return RegisterJSONGrammar(j)
}

// Make builds a standard-JSON parser instance, optionally layering extra
// options (e.g. info.Map/List/Text) over the base strict configuration.
func Make(extra ...tabnas.Options) *tabnas.Tabnas {
	// Build a BARE engine and apply the strict options through the plugin
	// (i.e. SetOptions), exactly as TS `make()` does via
	// `new Tabnas({plugins:[json]})`. Going through the one plugin entry
	// point is the point: the Make path and the Use(Json) path cannot
	// drift, so there is only one definition of "strict JSON".
	//
	// Historical note: this also used to be load-bearing. Engine versions
	// up to and including parser/go v0.6.1 applied only a subset of
	// Options in the tabnas.Make() constructor and silently dropped
	// Options.TokenSet, leaving KEY at the engine default
	// (#TX #NR #ST #VL) so `{1:1}` / `{null:null}` parsed. Current engine
	// versions apply TokenSet in Make() too, so both constructions now
	// agree — but a GOWORK=off build still resolves v0.6.1, so keep this
	// path until the engine republishes.
	j := tabnas.Make()
	if err := Json(j, nil); err != nil {
		// The grammar spec is fixed and valid, so this only fires on a
		// programmer error while editing the grammar.
		panic(err)
	}
	// Extra options are applied after the grammar exists so that rule
	// include/exclude filters operate on the installed alternates (and
	// info options reach the config the grammar closures captured).
	for _, o := range extra {
		j.SetOptions(o)
	}
	return j
}

// defaultParser is a lazily-created instance reused by Parse, so repeated
// calls don't rebuild the engine and grammar each time. Parsing builds a
// fresh context per call and only reads instance state, so the shared
// instance is safe for concurrent use.
var (
	defaultOnce   sync.Once
	defaultParser *tabnas.Tabnas
)

// Parse parses a JSON source string with a default standard-JSON parser
// and returns the resulting value, or a *tabnas.TabnasError on failure.
func Parse(src string) (any, error) {
	defaultOnce.Do(func() { defaultParser = Make() })
	return defaultParser.Parse(src)
}

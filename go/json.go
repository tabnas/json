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
	"sync"

	tabnas "github.com/tabnas/parser/go"
)

// Version is the current version of the module.
const Version = "0.2.0"

// strictNumber matches exactly a standard JSON number. Anything the
// engine's (lenient) number matcher accepts that does not match this —
// leading `+`, a bare leading `.` (".5"), a trailing `.` ("1."), leading
// zeros ("01", "00") — is excluded and so rejected.
var strictNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

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
			Exclude: func(s string) bool { return !strictNumber.MatchString(s) },
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
		Rule:    &tabnas.RuleOptions{Finish: &f},
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
				{S: "#CA", R: "pair", A: "@setval$", G: "map,pair,json"},
				{S: "#CB", B: 1, A: "@setval$", G: "map,pair,json"},
			},
		},
		// elem: a value inside a list. @push$ appends the built value.
		"elem": {
			Open: []*tabnas.GrammarAltSpec{
				{P: "val", G: "list,elem,val,json"},
			},
			Close: []*tabnas.GrammarAltSpec{
				{S: "#CA", R: "elem", A: "@push$", G: "list,elem,json"},
				{S: "#CS", B: 1, A: "@push$", G: "list,elem,json"},
			},
		},
	}

	return j.Grammar(&tabnas.GrammarSpec{V: 2, Rule: rules})
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
	j := tabnas.Make(jsonOptions())
	if err := RegisterJSONGrammar(j); err != nil {
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

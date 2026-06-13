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
package json

import (
	"regexp"

	tabnas "github.com/tabnas/parser/go"
)

// Version is the current version of the module.
const Version = "1.0.0"

// strictNumber matches exactly a standard JSON number. Anything the
// engine's (lenient) number matcher accepts that does not match this —
// leading `+`, a bare leading `.` (".5"), a trailing `.` ("1."), leading
// zeros ("01", "00") — is excluded and so rejected.
var strictNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// jsonMapSet / jsonListAppend write into either a plain map/slice or the
// MapRef/ListRef wrappers used when the info options are enabled.

func jsonMapSet(node any, key string, val any) any {
	if mr, ok := node.(tabnas.MapRef); ok {
		mr.Val[key] = val
		return mr
	}
	m, _ := node.(map[string]any)
	m[key] = val
	return m
}

func jsonListAppend(node any, val any) any {
	if lr, ok := node.(tabnas.ListRef); ok {
		lr.Val = append(lr.Val, val)
		return lr
	}
	s, _ := node.([]any)
	return append(s, val)
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
		// Strict JSON keys are quoted strings only.
		TokenSet: map[string][]string{"KEY": {"#ST"}},
	}
}

// RegisterJSONGrammar installs the standard JSON rule set (val / map /
// list / pair / elem) on j. Exposed separately from the options so other
// grammar plugins can layer on the JSON core. cfg is read for the info
// (MapRef / ListRef / Text) settings.
func RegisterJSONGrammar(j *tabnas.Tabnas) {
	cfg := j.Config()

	// val: a value is a map, a list, or a plain scalar token.
	j.Rule("val", func(rs *tabnas.RuleSpec, _ *tabnas.Parser) {
		rs.BO = []tabnas.StateAction{func(r *tabnas.Rule, ctx *tabnas.Context) {
			r.Node = tabnas.Undefined
		}}
		rs.BC = []tabnas.StateAction{func(r *tabnas.Rule, ctx *tabnas.Context) {
			// A map/list child node wins; otherwise the value is the scalar
			// token. (The strict lexer guarantees a value rule always has
			// one or the other — there are no empty values to coalesce.)
			if !tabnas.IsUndefined(r.Child.Node) {
				r.Node = r.Child.Node
				return
			}
			val := r.O0.ResolveVal(r, ctx)
			if cfg.TextInfo && (r.O0.Tin == tabnas.TinST || r.O0.Tin == tabnas.TinTX) {
				quote := ""
				if r.O0.Tin == tabnas.TinST && len(r.O0.Src) > 0 {
					quote = string(r.O0.Src[0])
				}
				str, _ := val.(string)
				val = tabnas.Text{Quote: quote, Str: str}
			}
			r.Node = val
		}}
		rs.Open = []*tabnas.AltSpec{
			{S: [][]tabnas.Tin{{tabnas.TinOB}}, P: "map", B: 1, G: "map,json"},
			{S: [][]tabnas.Tin{{tabnas.TinOS}}, P: "list", B: 1, G: "list,json"},
			{S: [][]tabnas.Tin{tabnas.TinSetVAL}, G: "val,json"},
		}
		rs.Close = []*tabnas.AltSpec{
			{S: [][]tabnas.Tin{{tabnas.TinZZ}}, G: "end,json"},
			{B: 1, G: "more,json"},
		}
	})

	// map: an object.
	j.Rule("map", func(rs *tabnas.RuleSpec, _ *tabnas.Parser) {
		rs.BO = []tabnas.StateAction{func(r *tabnas.Rule, ctx *tabnas.Context) {
			if cfg.MapRef {
				r.Node = tabnas.MapRef{Val: make(map[string]any), Meta: make(map[string]any)}
			} else {
				r.Node = make(map[string]any)
			}
		}}
		rs.BC = []tabnas.StateAction{func(r *tabnas.Rule, ctx *tabnas.Context) {
			if cfg.MapRef {
				if mr, ok := r.Node.(tabnas.MapRef); ok {
					mr.Implicit = !(r.O0 != tabnas.NoToken && r.O0.Tin == tabnas.TinOB)
					r.Node = mr
				}
			}
		}}
		rs.Open = []*tabnas.AltSpec{
			{S: [][]tabnas.Tin{{tabnas.TinOB}, {tabnas.TinCB}}, B: 1, N: map[string]int{"pk": 0}, G: "map,json"},
			{S: [][]tabnas.Tin{{tabnas.TinOB}}, P: "pair", N: map[string]int{"pk": 0}, G: "map,json,pair"},
		}
		rs.Close = []*tabnas.AltSpec{
			{S: [][]tabnas.Tin{{tabnas.TinCB}}, G: "end,json"},
		}
	})

	// list: an array.
	j.Rule("list", func(rs *tabnas.RuleSpec, _ *tabnas.Parser) {
		rs.BO = []tabnas.StateAction{func(r *tabnas.Rule, ctx *tabnas.Context) {
			if cfg.ListRef {
				r.Node = tabnas.ListRef{Val: make([]any, 0), Meta: make(map[string]any)}
			} else {
				r.Node = make([]any, 0)
			}
		}}
		rs.BC = []tabnas.StateAction{func(r *tabnas.Rule, ctx *tabnas.Context) {
			if cfg.ListRef {
				if lr, ok := r.Node.(tabnas.ListRef); ok {
					lr.Implicit = !(r.O0 != tabnas.NoToken && r.O0.Tin == tabnas.TinOS)
					r.Node = lr
				}
			}
		}}
		rs.Open = []*tabnas.AltSpec{
			{S: [][]tabnas.Tin{{tabnas.TinOS}, {tabnas.TinCS}}, B: 1, G: "list,json"},
			{S: [][]tabnas.Tin{{tabnas.TinOS}}, P: "elem", G: "list,elem,json"},
		}
		rs.Close = []*tabnas.AltSpec{
			{S: [][]tabnas.Tin{{tabnas.TinCS}}, G: "end,json"},
		}
	})

	// pair: a key:value entry inside a map.
	j.Rule("pair", func(rs *tabnas.RuleSpec, _ *tabnas.Parser) {
		rs.BC = []tabnas.StateAction{func(r *tabnas.Rule, ctx *tabnas.Context) {
			if _, ok := r.U["pair"]; ok {
				key, _ := r.U["key"].(string)
				r.Node = jsonMapSet(r.Node, key, r.Child.Node)
			}
		}}
		rs.Open = []*tabnas.AltSpec{
			{
				S: [][]tabnas.Tin{{tabnas.TinST}, {tabnas.TinCL}},
				P: "val",
				U: map[string]any{"pair": true},
				G: "map,pair,key,json",
				A: func(r *tabnas.Rule, ctx *tabnas.Context) {
					// Strict JSON keys are quoted strings (the KEY token set
					// is restricted to #ST), so the key value is the string.
					r.U["key"], _ = r.O0.Val.(string)
				},
			},
		}
		rs.Close = []*tabnas.AltSpec{
			{S: [][]tabnas.Tin{{tabnas.TinCA}}, R: "pair", G: "map,pair,json"},
			{S: [][]tabnas.Tin{{tabnas.TinCB}}, B: 1, G: "map,pair,json"},
		}
	})

	// elem: a value inside a list.
	j.Rule("elem", func(rs *tabnas.RuleSpec, _ *tabnas.Parser) {
		rs.BC = []tabnas.StateAction{func(r *tabnas.Rule, ctx *tabnas.Context) {
			if !tabnas.IsUndefined(r.Child.Node) {
				r.Node = jsonListAppend(r.Node, r.Child.Node)
				if r.Parent != tabnas.NoRule && r.Parent != nil {
					r.Parent.Node = r.Node
				}
			}
		}}
		rs.Open = []*tabnas.AltSpec{
			{P: "val", G: "list,elem,val,json"},
		}
		rs.Close = []*tabnas.AltSpec{
			{S: [][]tabnas.Tin{{tabnas.TinCA}}, R: "elem", G: "list,elem,json"},
			{S: [][]tabnas.Tin{{tabnas.TinCS}}, B: 1, G: "list,elem,json"},
		}
	})
}

// Json is the standard plugin form: apply the strict JSON options, then
// register the JSON grammar. Use it on a bare engine, or call Make.
func Json(j *tabnas.Tabnas, _ map[string]any) error {
	j.SetOptions(jsonOptions())
	RegisterJSONGrammar(j)
	return nil
}

// Make builds a standard-JSON parser instance, optionally layering extra
// options (e.g. info.Map/List/Text) over the base strict configuration.
func Make(extra ...tabnas.Options) *tabnas.Tabnas {
	j := tabnas.Make(jsonOptions())
	RegisterJSONGrammar(j)
	// Extra options are applied after the grammar exists so that rule
	// include/exclude filters operate on the installed alternates (and
	// info options reach the config the grammar closures captured).
	for _, o := range extra {
		j.SetOptions(o)
	}
	return j
}

// Parse parses a JSON source string with a default standard-JSON parser
// and returns the resulting value, or a *tabnas.TabnasError on failure.
func Parse(src string) (any, error) {
	return Make().Parse(src)
}

// Copyright (c) 2026 tabnas, MIT License

// Package debugtest is the debug-integration test module: it holds the
// optional test that layers the official @tabnas/debug plugin on the
// standard JSON grammar. It lives in its own module so the main package
// never depends on the external debug tool; see go.mod.
package tabnasdebugtest

import (
	"reflect"
	"strings"
	"testing"

	debug "github.com/tabnas/debug/go"
	tjson "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

// deorder recursively rewrites parsed objects from the engine's
// insertion-ordered *tabnas.OrderedMap into a plain map[string]any (walking
// slices too), so a parsed value can be compared by value with
// reflect.DeepEqual against a plain-map expectation. Only the object wrapper
// is dropped; scalars are carried through unchanged.
func deorder(v any) any {
	switch t := v.(type) {
	case *tabnas.OrderedMap:
		m := make(map[string]any, len(t.Keys))
		for _, k := range t.Keys {
			val, _ := t.Get(k)
			m[k] = deorder(val)
		}
		return m
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = deorder(val)
		}
		return m
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = deorder(val)
		}
		return out
	default:
		return v
	}
}

func TestJSONComposesWithDebug(t *testing.T) {
	j := tjson.Make()
	if err := j.Use(debug.Debug); err != nil {
		t.Fatalf("Use(debug.Debug): %v", err)
	}

	// Parsing still works with the debug plugin installed.
	out, err := j.Parse(`{"a":[1,2]}`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Parsed objects are now insertion-ordered *tabnas.OrderedMap; deorder
	// unwraps them to plain maps so the comparison is by value.
	want := map[string]any{"a": []any{float64(1), float64(2)}}
	if !reflect.DeepEqual(deorder(out), want) {
		t.Fatalf("Parse = %#v, want %#v", out, want)
	}

	// debug.Describe introspects the installed JSON grammar.
	desc, err := debug.Describe(j)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	for _, rule := range []string{"val", "map", "list", "pair", "elem"} {
		if !strings.Contains(desc, rule) {
			t.Fatalf("describe missing rule %q", rule)
		}
	}
}

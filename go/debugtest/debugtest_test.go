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
)

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
	want := map[string]any{"a": []any{float64(1), float64(2)}}
	if !reflect.DeepEqual(out, want) {
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

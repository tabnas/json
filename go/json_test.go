// Copyright (c) 2026 tabnas, MIT License

package json

import (
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tabnas "github.com/tabnas/parser/go"
)

// specDir is where the shared conformance fixtures live. TypeScript is
// canonical; the Go suite runs the same .tsv files to prove parity.
const specDir = "../ts/test/spec"

func loadTSV(t *testing.T, name string) [][2]string {
	t.Helper()
	path := filepath.Join(specDir, name+".tsv")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("spec file not found: %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")
	var rows [][2]string
	for i, line := range lines {
		if i == 0 || line == "" {
			continue // skip header and blank lines
		}
		cols := strings.SplitN(line, "\t", 2)
		if len(cols) != 2 {
			t.Fatalf("%s line %d: expected 2 columns", name, i+1)
		}
		rows = append(rows, [2]string{cols[0], cols[1]})
	}
	return rows
}

// canon marshals a value to canonical JSON for comparison.
func canon(t *testing.T, v any) string {
	t.Helper()
	b, err := stdjson.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestSpecValid(t *testing.T) {
	for _, row := range loadTSV(t, "json-valid") {
		input := row[0]
		t.Run(input, func(t *testing.T) {
			got, err := Parse(input)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", input, err)
			}
			// Cross-check against the standard library parser. DeepEqual
			// compares floats with ==, so -0 and 0 (both valid, both the
			// value zero) are treated as equal.
			var want any
			if err := stdjson.Unmarshal([]byte(input), &want); err != nil {
				t.Fatalf("encoding/json rejected valid fixture %q: %v", input, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Parse(%q) = %s, want %s", input, canon(t, got), canon(t, want))
			}
		})
	}
}

func TestSpecErrors(t *testing.T) {
	for _, row := range loadTSV(t, "json-errors") {
		input, code := row[0], row[1]
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input)
			if err == nil {
				t.Fatalf("Parse(%q) expected error %q, got nil", input, code)
			}
			je, ok := err.(*tabnas.TabnasError)
			if !ok {
				t.Fatalf("Parse(%q) returned %T, want *tabnas.TabnasError", input, err)
			}
			// The error code is part of the shared parity contract: both
			// runtimes must reject with the same code.
			if je.Code != code {
				t.Fatalf("Parse(%q) code = %q, want %q", input, je.Code, code)
			}
			// Sanity: the standard library must also reject it.
			var v any
			if stdjson.Unmarshal([]byte(input), &v) == nil {
				t.Fatalf("encoding/json accepted invalid fixture %q", input)
			}
		})
	}
}

func TestScalars(t *testing.T) {
	cases := map[string]any{
		"42":    float64(42),
		"-3.14": float64(-3.14),
		`"x"`:   "x",
		"true":  true,
		"false": false,
		"null":  nil,
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Parse(%q) = %#v, want %#v", in, got, want)
		}
	}
}

func TestSurrogatePair(t *testing.T) {
	got, err := Parse(`"😀"`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "😀" {
		t.Fatalf("got %q, want emoji", got)
	}
}

func TestPluginIsUsable(t *testing.T) {
	j := tabnas.Make()
	if err := j.Use(Json); err != nil {
		t.Fatalf("Use(Json): %v", err)
	}
	got, err := j.Parse(`{"a":[1,2,3]}`)
	if err != nil {
		t.Fatal(err)
	}
	if canon(t, got) != `{"a":[1,2,3]}` {
		t.Fatalf("got %s", canon(t, got))
	}
}

func TestRejectsExtendedGrammar(t *testing.T) {
	// Inputs jsonic accepts but standard JSON does not.
	for _, in := range []string{
		"{a:1}",     // unquoted key
		"[1,2,]",    // trailing comma
		"1 // note", // comment
		"'x'",       // single quotes
		"a:1,b:2",   // implicit object
		"x,y,z",     // implicit array
		"0x10",      // hex number
		".5",        // bare leading dot
		"+1",        // leading plus
		"1.",        // trailing dot
		"01",        // leading zero
		"",          // empty input
		"   ",       // whitespace only
	} {
		if _, err := Parse(in); err == nil {
			t.Fatalf("Parse(%q) expected error, got nil", in)
		}
	}
}

// Copyright (c) 2026 tabnas, MIT License

package tabnasjson

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

func TestInfoOptions(t *testing.T) {
	// Enabling Info.Map/List/Text exercises the MapRef/ListRef/Text
	// branches the plain-JSON config leaves off (and the Make extra-options
	// path); kept so other plugins can build on this grammar.
	tr := true
	j := Make(tabnas.Options{Info: &tabnas.InfoOptions{Map: &tr, List: &tr, Text: &tr}})
	out, err := j.Parse(`{"a":["x",1]}`)
	if err != nil {
		t.Fatal(err)
	}
	mr, ok := out.(tabnas.MapRef)
	if !ok {
		t.Fatalf("want MapRef, got %T", out)
	}
	if mr.Implicit {
		t.Error("explicit map marked implicit")
	}
	lr, ok := mr.Val["a"].(tabnas.ListRef)
	if !ok {
		t.Fatalf("want ListRef, got %T", mr.Val["a"])
	}
	if lr.Implicit {
		t.Error("explicit list marked implicit")
	}
	if len(lr.Val) != 2 {
		t.Fatalf("list len = %d, want 2", len(lr.Val))
	}
	tx, ok := lr.Val[0].(tabnas.Text)
	if !ok {
		t.Fatalf("want Text, got %T", lr.Val[0])
	}
	if tx.Quote != `"` || tx.Str != "x" {
		t.Fatalf("text = %+v", tx)
	}
}

func TestComposeJSONC(t *testing.T) {
	// The JSON grammar is a foundation: layering comment lexing on top
	// yields a JSON-with-comments parser (the documented example).
	tr := true
	jc := Make(tabnas.Options{Comment: &tabnas.CommentOptions{Lex: &tr}})
	for _, s := range []string{`{"a":1} // note`, `{"a":/* x */1}`} {
		out, err := jc.Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q): %v", s, err)
		}
		if canon(t, out) != `{"a":1}` {
			t.Fatalf("Parse(%q) = %s, want {\"a\":1}", s, canon(t, out))
		}
	}
	// The base json parser still rejects comments.
	if _, err := Parse(`{"a":1}//c`); err == nil {
		t.Fatal("base json accepted a comment")
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
		`"\x41"`,    // \xHH ascii escape
		`"\u{41}"`,  // \u{...} braced escape
		`"\v"`,      // non-standard \v escape
		`"\'"`,      // non-standard \' escape
		"\"\\`\"",   // non-standard backtick escape
		"",          // empty input
		"   ",       // whitespace only
	} {
		if _, err := Parse(in); err == nil {
			t.Fatalf("Parse(%q) expected error, got nil", in)
		}
	}
}

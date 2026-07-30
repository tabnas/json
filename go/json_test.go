// Copyright (c) 2026 tabnas, MIT License

package tabnasjson

import (
	stdjson "encoding/json"
	"errors"
	"reflect"
	"testing"

	tabnas "github.com/tabnas/parser/go"
)

// canon marshals a value to canonical JSON for comparison.
func canon(t *testing.T, v any) string {
	t.Helper()
	b, err := stdjson.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// deorder recursively rewrites parsed objects from the engine's
// insertion-ordered *tabnas.OrderedMap into a plain map[string]any (and
// walks slices), so a value produced by Parse can be compared by value with
// reflect.DeepEqual against encoding/json's plain-map output. Only the
// object WRAPPER is dropped; scalar values (including negative zero) are
// carried through unchanged, so the existing -0 == 0 float semantics hold.
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

// TestSpecValidOrder pins the migration's contract: a parsed object is an
// insertion-ordered *tabnas.OrderedMap that preserves SOURCE key order (not
// alphabetical), and re-marshals in that same order. The fixture's keys are
// deliberately out of alphabetical order so the two orders differ.
func TestSpecValidOrder(t *testing.T) {
	got, err := Parse(`{"b":1,"a":2,"c":3}`)
	if err != nil {
		t.Fatal(err)
	}
	om, ok := got.(*tabnas.OrderedMap)
	if !ok {
		t.Fatalf("parsed object = %T, want *tabnas.OrderedMap", got)
	}
	if want := []string{"b", "a", "c"}; !reflect.DeepEqual(om.Keys, want) {
		t.Fatalf("Keys = %v, want %v (source order)", om.Keys, want)
	}
	// MarshalJSON must emit keys in source order, not alphabetical.
	if s := canon(t, got); s != `{"b":1,"a":2,"c":3}` {
		t.Fatalf("marshal = %s, want source-order keys", s)
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

// Mirrors the TS JSON_OPTIONS `rule: { finish: false, include: 'json' }`:
// the Go options carry both Finish=false and Include="json".
func TestOptionsRuleInclude(t *testing.T) {
	j := Make()
	opts := j.Options()
	if opts.Rule == nil {
		t.Fatal("Options().Rule is nil")
	}
	if opts.Rule.Include != "json" {
		t.Fatalf("Options().Rule.Include = %q, want %q", opts.Rule.Include, "json")
	}
	if opts.Rule.Finish == nil || *opts.Rule.Finish {
		t.Fatal("Options().Rule.Finish should be false")
	}
	// The JSON grammar tags every alternate "json", so the include filter
	// keeps the full grammar working.
	got, err := j.Parse(`{"a":[1,2,3]}`)
	if err != nil {
		t.Fatal(err)
	}
	if canon(t, got) != `{"a":[1,2,3]}` {
		t.Fatalf("got %s", canon(t, got))
	}
}

// Mirrors the TS re-export `export { TabnasError as JsonError }`: a
// failed parse yields an error reachable as *JsonError.
func TestJsonErrorAlias(t *testing.T) {
	_, err := Parse("{")
	if err == nil {
		t.Fatal(`Parse("{") expected error, got nil`)
	}
	var je *JsonError
	if !errors.As(err, &je) {
		t.Fatalf(`Parse("{") error %T is not a *JsonError`, err)
	}
	if je.Code == "" {
		t.Fatal("JsonError.Code is empty")
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

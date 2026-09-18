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

// TestNumberOverflowRejected pins the Go half at encoding/json parity for
// out-of-range exponents. These are syntactically valid JSON, and the two
// platform oracles disagree: JSON.parse saturates to Infinity (so the TS
// half accepts) while encoding/json fails. The engine saturates to ±Inf to
// match TS, so the rejection lives in this plugin's number Exclude hook.
//
// The conformance suite covers this via its i_number_*_overflow cases, but
// that corpus is fetched and gitignored, so it does not run in CI. This
// test does.
func TestNumberOverflowRejected(t *testing.T) {
	reject := []string{
		"1e999", "-1e999", "1e+9999", "123123e100000", "-123123e100000",
		"1.5e+9999", "[1e999]", `{"a":1e999}`,
	}
	for _, src := range reject {
		if _, err := Parse(src); err == nil {
			t.Errorf("%s: expected rejection (encoding/json rejects), got nil error", src)
		}
	}

	// In range, or underflow, which encoding/json accepts.
	accept := []string{"1e308", "-1e308", "1e-999", "-1e-999", "0e0", "1e2"}
	for _, src := range accept {
		if _, err := Parse(src); err != nil {
			t.Errorf("%s: expected acceptance, got %v", src, err)
		}
	}
}

// pushChainOf reads push$.chain off the given `elem` close alt of the
// grammar INSTALLED on j, and reports whether the alt carries it at all.
func pushChainOf(t *testing.T, j *tabnas.Tabnas, i int) (chain bool, set bool) {
	t.Helper()
	elem := j.RSM()["elem"]
	if elem == nil {
		t.Fatalf("engine has no elem rule")
	}
	alts := elem.CloseAlts()
	if len(alts) <= i {
		t.Fatalf("elem has %d close alts, wanted index %d", len(alts), i)
	}
	k := alts[i].K
	if k == nil {
		return false, false
	}
	cfg, ok := k["push$"].(map[string]any)
	if !ok {
		t.Fatalf("elem close alt %d has K but no push$ config: %#v", i, k)
	}
	v, ok := cfg["chain"]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	if !ok {
		t.Fatalf("elem close alt %d push$.chain is %T, wanted bool", i, v)
	}
	return b, true
}

// The reusable core is the entry point other plugins layer on, and
// push$.chain: false is a claim about the ASSEMBLED grammar -- that
// nothing in it resolves $prev to read a rule `R: "elem"` replaced. The
// core cannot make that claim for rules it has never seen, so bare
// RegisterJSONGrammar must leave the key off and let the engine walk.
//
// These read the installed grammar back rather than compare parses,
// because the pinned engine release ignores push$.chain: both grammars
// parse identically today. They stop being identical the moment go.mod
// moves to a release that honours the key, which is exactly when a
// layered plugin reading $prev would start getting a silent wrong answer
// in Go and the right one in TypeScript and Rust.
func TestRulesOnlyInstallerLeavesTheChainWalkOn(t *testing.T) {
	j := tabnas.Make(tabnas.Options{})
	if err := RegisterJSONGrammar(j); err != nil {
		t.Fatalf("RegisterJSONGrammar: %v", err)
	}
	for i := range 2 {
		if _, set := pushChainOf(t, j, i); set {
			t.Errorf("elem close alt %d: the layerable core must not set push$.chain", i)
		}
	}
}

// A layering plugin that knows its own rules never read a replaced rule
// can still ask for the optimization. This is the opt-in half of the same
// contract, and the shape Json uses.
func TestRulesOnlyInstallerHonoursChainOff(t *testing.T) {
	j := tabnas.Make(tabnas.Options{})
	if err := RegisterJSONGrammar(j, GrammarOptions{ChainOff: true}); err != nil {
		t.Fatalf("RegisterJSONGrammar: %v", err)
	}
	for i := range 2 {
		chain, set := pushChainOf(t, j, i)
		if !set {
			t.Errorf("elem close alt %d: ChainOff did not set push$.chain", i)
			continue
		}
		if chain {
			t.Errorf("elem close alt %d: push$.chain is true, wanted false", i)
		}
	}
}

// Json IS the assembled grammar -- these rules are all the rules, and
// none of them reads a replaced rule -- so it is the one caller that can
// honestly opt in, and it does. Wiring Json back to the bare installer
// would silently hand the shipped parser its O(elements^2) walk again;
// this fails instead of only a benchmark moving.
func TestJsonPluginOptsOutOfTheChainWalk(t *testing.T) {
	j := tabnas.Make(tabnas.Options{})
	if err := Json(j, nil); err != nil {
		t.Fatalf("Json: %v", err)
	}
	for i := range 2 {
		chain, set := pushChainOf(t, j, i)
		if !set {
			t.Errorf("elem close alt %d: Json did not opt out of the chain walk", i)
			continue
		}
		if chain {
			t.Errorf("elem close alt %d: push$.chain is true, wanted false", i)
		}
	}
	// And it still parses a list, opt-out and all.
	got, err := j.Parse(`[1,2,3]`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if want := `[1,2,3]`; canon(t, got) != want {
		t.Errorf("Json parse = %s, want %s", canon(t, got), want)
	}
}

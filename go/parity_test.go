// Copyright (c) 2026 tabnas, MIT License

package tabnasjson

// parity_test.go — cross-runtime conformance, driven by the shared
// `test/spec/*.tsv` fixtures at the repo root (see ../test/AGENTS.md).
//
// The fixture loader, the escape codec, the ERROR:<code> contract and the
// row loop all come from github.com/tabnas/support/go, whose TypeScript
// half ts/test/parity.test.js uses to run the SAME files — so the two
// implementations cannot drift without one of them going red, and neither
// can the two loaders.
//
// What is left here is only what is specific to @tabnas/json: every row is
// also cross-checked against encoding/json, since this package's contract
// IS plain JSON.

import (
	stdjson "encoding/json"
	"fmt"
	"testing"

	tabnas "github.com/tabnas/parser/go"
	support "github.com/tabnas/support/go"
)

// TestSpec runs every fixture in the spec directory. FindSpecDir walks up
// from the package directory, and Dir discovers the files by listing, so
// adding a .tsv runs it in both runtimes without touching either runner.
func TestSpec(t *testing.T) {
	dir, err := support.FindSpecDir("")
	if err != nil {
		t.Fatal(err)
	}

	support.Runner{
		Parse: func(input string) (any, error) {
			got, err := Parse(input)
			if err != nil {
				return nil, err
			}

			// The standard library must accept it, and produce the same
			// value. Returning the error surfaces as a failed row, naming
			// the fixture and line.
			var want any
			if err := stdjson.Unmarshal([]byte(input), &want); err != nil {
				return nil, fmt.Errorf(
					"encoding/json rejected a valid fixture: %w", err)
			}
			if !support.EqualValueWith(got, want, deorder) {
				return nil, fmt.Errorf(
					"disagrees with encoding/json: %s != %s",
					support.FormatValue(deorder(got)), support.FormatValue(want))
			}

			return got, nil
		},

		// Parsed objects are insertion-ordered *tabnas.OrderedMap; deorder
		// unwraps them so the comparison is by value. Key order is
		// separately covered by TestSpecValidOrder — TypeScript pins it in
		// this suite instead, by comparing the rendered text.
		Normalize: deorder,

		// Two sanity checks the code comparison would not otherwise make.
		// Each answers a pseudo-code rather than failing here, so the
		// failure reads as "failed with code <what went wrong>, expected
		// <the row's code>" and names the row.
		ErrorCode: func(err error) string {
			je, ok := err.(*tabnas.TabnasError)
			if !ok {
				return fmt.Sprintf("not-a-TabnasError(%T)", err)
			}
			return je.Code
		},
	}.Dir(t, dir)
}

// TestSpecRejectedByBoth pins the other half of the plain-JSON contract:
// every input a fixture says must FAIL must be rejected by encoding/json
// too. It is a separate walk rather than part of the runner's error branch
// because the runner hands the hook an error, not the input that caused it.
func TestSpecRejectedByBoth(t *testing.T) {
	dir, err := support.FindSpecDir("")
	if err != nil {
		t.Fatal(err)
	}

	specs, err := support.LoadSpecDir(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, spec := range specs {
		for _, row := range spec.Rows {
			if !support.IsErrorExpect(row.Col(1)) {
				continue
			}
			checked++

			input := row.Unesc(0)
			var sink any
			if nil == stdjson.Unmarshal([]byte(input), &sink) {
				t.Errorf("%s: encoding/json accepted %q, which the fixture "+
					"says must be rejected", row.Where(), input)
			}
		}
	}

	// A cross-check that walked no rows would be green having asserted
	// nothing, which is the failure mode this whole suite guards against.
	if 0 == checked {
		t.Fatal("no ERROR rows found — the cross-check asserted nothing")
	}
}

// TypeScript is canonical, and it reports this grammar's rules in the order
// they are declared: val, map, list, pair, elem. A Go map has no order, so
// without GrammarSpec.RuleOrder the engine falls back to sorted names and
// RuleNames answers [elem list map pair val] instead. That is not cosmetic:
// railroad's extracted Go model, and anything else built on RuleNames, renders
// in whatever order this returns.
//
// Hard-coded rather than derived, deliberately. Reading the order back out of
// the same literal that sets it would assert nothing; this is a copy of what
// ts/src/json.ts declares, so it fails if either side moves.
func TestRuleOrderMatchesTypeScriptDeclarationOrder(t *testing.T) {
	want := []string{"val", "map", "list", "pair", "elem"}
	got := Make().RuleNames()

	if len(got) != len(want) {
		t.Fatalf("RuleNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("RuleNames() = %v, want %v (differs at %d)", got, want, i)
		}
	}
}

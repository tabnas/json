/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

package tabnasjson

// Negative zero, pinned per port because the shared fixture cannot hold it.
//
// `-0` used to be a row in test/spec/valid.tsv. It cannot stay there, and
// the reason is worth writing down rather than rediscovering.
//
// The two halves of that fixture ask DIFFERENT QUESTIONS of the `expected`
// column. The TypeScript runner (ts/test/parity.test.js) compares the
// RENDERING — `JSON.stringify(parse(input))` against the cell — which is
// deliberate, because comparing text pins key ORDER as well as value. The
// Go runner compares the VALUE, and pins order separately in
// TestSpecValidOrder.
//
// For every other input those two questions have the same answer. For
// negative zero they do not, because the renderings differ:
//
//     JS    JSON.stringify(-0)      "0"     — the sign is lost
//     Go    json.Marshal(-0)        "-0"    — the sign survives
//
// So a text-compared cell must say `0` for TypeScript and `-0` for Go, and
// one column cannot be both. The row was green until tabnas/support#13 put
// signed zero in the value contract, at which point Go's value comparison
// started distinguishing `0` from `-0` and the cell became wrong for Go
// while staying right for TypeScript.
//
// The VALUE fact is not in dispute — both ports parse `-0` to negative
// zero — so it is pinned here and in ts/test/signed-zero.test.js instead,
// where each port can assert it in terms its own runtime can express.
// ADR-15: signed zero is in the value contract.

import (
	"math"
	"testing"
)

func TestParsesNegativeZero(t *testing.T) {
	for _, c := range []struct {
		src  string
		neg  bool
		want float64
	}{
		// The subject.
		{"-0", true, 0},

		// Controls. Without them "distinguishes signed zero" is also
		// satisfied by reporting every zero as negative, or by never
		// looking at the sign at all.
		{"0", false, 0},
		{"-0.0", true, 0},
		{"0.0", false, 0},
		{"-1", true, -1},
	} {
		v, err := Parse(c.src)
		if err != nil {
			t.Errorf("%s: %v", c.src, err)
			continue
		}
		f, ok := v.(float64)
		if !ok {
			t.Errorf("%s: parsed as %T, want float64", c.src, v)
			continue
		}
		if f != c.want {
			t.Errorf("%s: magnitude = %v, want %v", c.src, f, c.want)
		}
		if got := math.Signbit(f); got != c.neg {
			t.Errorf("%s: signbit = %v, want %v", c.src, got, c.neg)
		}
	}
}

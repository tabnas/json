// Copyright (c) 2026 tabnas, MIT License

package tabnasjson

// conformance_test.go — the external RFC 8259 conformance suite.
//
// Runs nst/JSONTestSuite (https://github.com/nst/JSONTestSuite), the
// standard cross-implementation JSON parsing suite, against this package.
//
// The suite is not vendored; it is fetched at a pinned commit into the
// .gitignore'd test/jsontestsuite/ by test/fetch-jsontestsuite.sh, which
// TestMain below runs when the corpus is absent — so a plain
// `go test ./...`, locally and in CI, always grades against it.
//
// If the corpus cannot be obtained these tests FAIL rather than skip. A
// conformance suite that quietly does not run reports a green tick that is
// a lie: it says "RFC 8259 conformant" while measuring nothing.
//
// The suite's file-name prefixes are the contract:
//
//	y_  MUST be accepted by a conforming parser — and must produce the same
//	    VALUE as the platform parser, not merely "it did not error". The
//	    oracle, encoding/json, is independent of this package, so the
//	    assertion is not circular.
//	n_  MUST be rejected by a conforming parser, with an error code.
//	i_  implementation-defined; this package's contract is parity with the
//	    platform parser, so we assert we agree with encoding/json.
//
// ts/test/conformance.test.js runs the SAME directory with the same rules,
// so the two runtimes cannot drift on it.

import (
	stdjson "encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	tabnas "github.com/tabnas/parser/go"
)

// Exact shape of the corpus at the pinned commit
// (1ef36fa01286573e846ac449e8683f8833c5b26a); see
// test/fetch-jsontestsuite.sh. Asserted before grading, so narrowing the
// corpus goes red instead of inflating the pass rate.
const (
	expectAccept = 95  // y_
	expectReject = 188 // n_
	expectImpl   = 35  // i_
	expectCases  = 318
)

const missingCorpus = `nst/JSONTestSuite corpus not found at %s

It is third-party and deliberately not vendored. Fetch it (pinned commit,
idempotent) and re-run:

    sh test/fetch-jsontestsuite.sh    # or: make json-test-suite

This fails rather than skips on purpose: a conformance suite that quietly
does not run reports a green tick that is a lie.`

// TestMain fetches the corpus when it is not already on disk, so a bare
// `go test ./...` — which is all CI runs, with no repo-specific step —
// grades against it. The fetch is pinned and idempotent; if it cannot run,
// the tests below fail loudly rather than skip.
func TestMain(m *testing.M) {
	if _, err := os.Stat(conformanceDir()); err != nil {
		script := filepath.Join("..", "test", "fetch-jsontestsuite.sh")
		cmd := exec.Command("sh", script)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr,
				"warning: %s failed (%v); the conformance tests will fail\n",
				script, err)
		}
	}
	os.Exit(m.Run())
}

func conformanceDir() string {
	return filepath.Join("..", "test", "jsontestsuite", "test_parsing")
}

// loadConformance returns the sorted case file names, failing loudly (never
// skipping) when the corpus is absent or has been narrowed.
func loadConformance(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(conformanceDir())
	if err != nil {
		t.Fatalf(missingCorpus, conformanceDir())
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// TestConformanceCorpusIntact — the corpus is exactly the pinned one, so a
// half-cloned or narrowed directory cannot pass vacuously.
func TestConformanceCorpusIntact(t *testing.T) {
	names := loadConformance(t)
	var y, n, i int
	for _, name := range names {
		switch {
		case strings.HasPrefix(name, "y_"):
			y++
		case strings.HasPrefix(name, "n_"):
			n++
		case strings.HasPrefix(name, "i_"):
			i++
		}
	}
	if y != expectAccept || n != expectReject || i != expectImpl ||
		len(names) != expectCases {
		t.Fatalf("corpus has been narrowed or replaced under %s: "+
			"y_=%d n_=%d i_=%d total=%d, want y_=%d n_=%d i_=%d total=%d — "+
			"re-run `sh test/fetch-jsontestsuite.sh --force`",
			conformanceDir(), y, n, i, len(names),
			expectAccept, expectReject, expectImpl, expectCases)
	}
}

func conformanceCase(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(conformanceDir(), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}

// TestConformanceMustAccept — every y_ case parses, AND yields the value
// encoding/json yields. "It did not error" is not conformance: a parser that
// accepts the input and returns the wrong value is silently losing data.
func TestConformanceMustAccept(t *testing.T) {
	for _, name := range loadConformance(t) {
		if !strings.HasPrefix(name, "y_") {
			continue
		}
		src := conformanceCase(t, name)
		// The oracle. If the standard library rejects a y_ case the corpus
		// is wrong rather than this parser — worth failing loudly for too.
		var want any
		if err := stdjson.Unmarshal(src, &want); err != nil {
			t.Errorf("%s: encoding/json rejected a y_ case: %v", name, err)
			continue
		}
		got, err := Parse(string(src))
		if err != nil {
			t.Errorf("%s: must-accept case rejected: %v", name, err)
			continue
		}
		if !reflect.DeepEqual(deorder(got), want) {
			t.Errorf("%s: Parse = %v, encoding/json = %v", name, got, want)
		}
	}
}

// TestConformanceMustReject — every n_ case is rejected, with a code.
func TestConformanceMustReject(t *testing.T) {
	for _, name := range loadConformance(t) {
		if !strings.HasPrefix(name, "n_") {
			continue
		}
		src := conformanceCase(t, name)
		got, err := Parse(string(src))
		if err == nil {
			t.Errorf("%s: must-reject case accepted: %v", name, got)
		} else if je, ok := err.(*tabnas.TabnasError); !ok || je.Code == "" {
			t.Errorf("%s: rejected without an error code: %T %v", name, err, err)
		}
		// Sanity: the standard library must reject it too.
		var sink any
		if stdjson.Unmarshal(src, &sink) == nil {
			t.Errorf("%s: encoding/json accepted an n_ case", name)
		}
	}
}

// TestConformanceImplementationDefined — every i_ case agrees with
// encoding/json on accept vs reject, and on the parsed value.
//
// Value comparison is skipped for sources that are not valid UTF-8:
// encoding/json substitutes U+FFFD for invalid bytes while this parser
// preserves the source bytes verbatim (as the TS runtime does with the
// code units it is handed). Both still accept, which is what the RFC
// leaves implementation-defined.
func TestConformanceImplementationDefined(t *testing.T) {
	for _, name := range loadConformance(t) {
		if !strings.HasPrefix(name, "i_") {
			continue
		}
		src := conformanceCase(t, name)
		got, ourErr := Parse(string(src))
		var std any
		stdErr := stdjson.Unmarshal(src, &std)

		if (ourErr == nil) != (stdErr == nil) {
			t.Errorf("%s: ours err=%v, encoding/json err=%v", name, ourErr, stdErr)
			continue
		}
		if ourErr != nil || !utf8.Valid(src) {
			continue
		}
		if !reflect.DeepEqual(deorder(got), std) {
			t.Errorf("%s: Parse = %v, encoding/json = %v", name, got, std)
		}
	}
}

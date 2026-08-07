// Copyright (c) 2026 tabnas, MIT License

package tabnasjson

// jsontestsuite_test.go -- third-party RFC 8259 / ECMA-404 conformance:
// nst/JSONTestSuite, the corpus behind "Parsing JSON is a Minefield"
// (http://seriot.ch/projects/parsing_json.html).
//
// The corpus is NOT vendored. `scripts/fetch-jsontestsuite.sh` clones it at a
// pinned commit into the gitignored `test/jsontestsuite/`.
// `ts/test/jsontestsuite.test.js` grades the SAME files, so the two runtimes
// can be compared case for case.
//
// This suite MUST NOT be able to skip. If the corpus is absent every test
// here FAILS with instructions -- a conformance test that quietly does not
// run is worse than no test at all, because the green tick is a lie.
//
// Grading (see the upstream README):
//
//	y_  MUST be accepted -- and must produce the CORRECT VALUE, not merely
//	    "it did not error". The oracle is the platform parser,
//	    encoding/json: independent of this package, so the assertion is
//	    not circular.
//	n_  MUST be rejected, with a *tabnas.TabnasError carrying a code.
//	i_  implementation-defined by RFC 8259. This package's declared bar is
//	    parity with the platform parser (README / AGENTS.md rule 4), so i_
//	    is graded against encoding/json -- same accept/reject decision, and
//	    the same value when accepted.
//
// Unlike the TypeScript runner, Go strings hold arbitrary bytes, so the
// invalid-UTF-8 cases are fed to the parser verbatim rather than through a
// U+FFFD-replacing decode. That is a real difference between the two
// runtimes, and it is reported rather than papered over.

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

	tabnas "github.com/tabnas/parser/go"
)

// TestMain fetches the corpus if it is not already on disk, so a plain
// `go test ./...` (locally or in CI, which runs `go test -v ./...` and has no
// repo-specific step) always grades against it. The fetch is idempotent and
// pinned; if it cannot run, the tests below FAIL LOUDLY rather than skip.
func TestMain(m *testing.M) {
	if _, err := os.Stat(suiteDir()); err != nil {
		script := filepath.Join("..", "scripts", "fetch-jsontestsuite.sh")
		cmd := exec.Command("bash", script)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s failed (%v); "+
				"the JSONTestSuite conformance tests will fail\n", script, err)
		}
	}
	os.Exit(m.Run())
}

// Exact shape of the corpus at the pinned commit
// (1ef36fa01286573e846ac449e8683f8833c5b26a). Re-asserted before grading, so
// quietly narrowing the corpus goes red instead of inflating the pass rate.
const (
	expectY     = 95
	expectN     = 188
	expectI     = 35
	expectTotal = 318
)

func suiteDir() string {
	return filepath.Join("..", "test", "jsontestsuite", "test_parsing")
}

const missingCorpus = `nst/JSONTestSuite corpus not found at %s

The conformance corpus is third-party and is deliberately NOT vendored.
Fetch it (pinned commit, idempotent) and re-run:

    scripts/fetch-jsontestsuite.sh

This test FAILS rather than skips on purpose: a conformance suite that
quietly does not run reports a green tick that is a lie.`

// suiteFiles returns the corpus file names with the given prefix, failing
// loudly (never skipping) when the corpus is absent.
func suiteFiles(t *testing.T, prefix string) []string {
	t.Helper()
	dir := suiteDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf(missingCorpus, dir)
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".json") && strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func suiteSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(suiteDir(), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

func TestJSONTestSuiteCorpusIntact(t *testing.T) {
	y := len(suiteFiles(t, "y_"))
	n := len(suiteFiles(t, "n_"))
	i := len(suiteFiles(t, "i_"))
	all := len(suiteFiles(t, ""))
	if y != expectY || n != expectN || i != expectI || all != expectTotal {
		t.Fatalf("corpus has been narrowed or replaced: y_=%d n_=%d i_=%d total=%d, "+
			"want y_=%d n_=%d i_=%d total=%d -- re-run scripts/fetch-jsontestsuite.sh",
			y, n, i, all, expectY, expectN, expectI, expectTotal)
	}
}

// TestJSONTestSuiteMustAccept: y_ cases must parse AND yield the value the
// platform parser yields.
func TestJSONTestSuiteMustAccept(t *testing.T) {
	for _, name := range suiteFiles(t, "y_") {
		t.Run(name, func(t *testing.T) {
			src := suiteSource(t, name)

			var want any
			if err := stdjson.Unmarshal([]byte(src), &want); err != nil {
				// The oracle rejected a must-accept case: the corpus or the
				// stdlib disagrees with the RFC. Fail loudly either way.
				t.Fatalf("%s: encoding/json rejected a y_ case: %v", name, err)
			}

			got, err := Parse(src)
			if err != nil {
				t.Fatalf("%s: rejected input RFC 8259 requires a parser to accept: %v",
					name, err)
			}
			if !reflect.DeepEqual(deorder(got), want) {
				t.Fatalf("%s: value differs from encoding/json:\n got  %s\n want %s",
					name, canon(t, got), canon(t, want))
			}
		})
	}
}

// TestJSONTestSuiteMustReject: n_ cases must be rejected with a coded error.
func TestJSONTestSuiteMustReject(t *testing.T) {
	for _, name := range suiteFiles(t, "n_") {
		t.Run(name, func(t *testing.T) {
			src := suiteSource(t, name)
			got, err := Parse(src)
			if err == nil {
				t.Fatalf("%s: accepted input RFC 8259 requires a parser to reject; "+
					"parsed as %s", name, canon(t, got))
			}
			te, ok := err.(*tabnas.TabnasError)
			if !ok {
				t.Fatalf("%s: got %T, want *tabnas.TabnasError", name, err)
			}
			if te.Code == "" {
				t.Fatalf("%s: TabnasError has no code", name)
			}
		})
	}
}

// TestJSONTestSuiteImplementationDefined: i_ cases are left open by the RFC,
// but not by this package -- its declared contract is platform parity, so
// encoding/json decides both the accept/reject call and the value.
func TestJSONTestSuiteImplementationDefined(t *testing.T) {
	for _, name := range suiteFiles(t, "i_") {
		t.Run(name, func(t *testing.T) {
			src := suiteSource(t, name)

			var want any
			platformOK := stdjson.Unmarshal([]byte(src), &want) == nil

			got, err := Parse(src)
			ok := err == nil

			if ok != platformOK {
				verb := map[bool]string{true: "accepted", false: "rejected"}
				t.Fatalf("%s: this parser %s but encoding/json %s",
					name, verb[ok], verb[platformOK])
			}
			if platformOK && !reflect.DeepEqual(deorder(got), want) {
				t.Fatalf("%s: value differs from encoding/json:\n got  %s\n want %s",
					name, canon(t, got), canon(t, want))
			}
		})
	}
}

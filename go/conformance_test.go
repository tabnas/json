// Copyright (c) 2026 tabnas, MIT License

package tabnasjson

// conformance_test.go — the external RFC 8259 conformance suite.
//
// Runs nst/JSONTestSuite (https://github.com/nst/JSONTestSuite), the
// standard cross-implementation JSON parsing suite, against this package.
// The suite is not vendored; fetch it with `sh test/fetch-jsontestsuite.sh`
// (or `make json-test-suite`) and these tests light up. Without it they
// skip, and the RFC 8259 claim is unverified.
//
// The suite's file-name prefixes are the contract:
//
//	y_  MUST be accepted by a conforming parser
//	n_  MUST be rejected by a conforming parser
//	i_  implementation-defined; this package's contract is parity with the
//	    platform parser, so we assert we agree with encoding/json.
//
// ts/test/conformance.test.js runs the SAME directory with the same rules,
// so the two runtimes cannot drift on it.

import (
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	tabnas "github.com/tabnas/parser/go"
)

// The suite has 318 cases today; a floor well under that still catches a
// half-cloned or empty directory passing vacuously.
const minConformanceCases = 300

func conformanceDir() string {
	return filepath.Join("..", "test", "jsontestsuite", "test_parsing")
}

// loadConformance returns the sorted case file names, or skips the test when
// the suite has not been fetched.
func loadConformance(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(conformanceDir())
	if err != nil {
		t.Skip("JSONTestSuite not fetched — run `make json-test-suite`")
	}
	names := make([]string, 0, len(ents))
	for _, e := range ents {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) < minConformanceCases {
		t.Fatalf("found %d cases under %s, expected >= %d",
			len(names), conformanceDir(), minConformanceCases)
	}
	return names
}

func conformanceCase(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(conformanceDir(), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}

// TestConformanceMustAccept — every y_ case parses.
func TestConformanceMustAccept(t *testing.T) {
	for _, name := range loadConformance(t) {
		if !strings.HasPrefix(name, "y_") {
			continue
		}
		src := conformanceCase(t, name)
		if _, err := Parse(string(src)); err != nil {
			t.Errorf("%s: must-accept case rejected: %v", name, err)
		}
		// Sanity: the standard library must accept it too.
		var sink any
		if err := stdjson.Unmarshal(src, &sink); err != nil {
			t.Errorf("%s: encoding/json rejected a y_ case: %v", name, err)
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

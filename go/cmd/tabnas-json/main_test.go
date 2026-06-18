// Copyright (c) 2026 tabnas, MIT License

// Port of ts/test/cli.test.js: exercises the run/runMain CLI behavior
// (pretty-print on success, formatted error + exit 1 on bad input,
// argument parsing, and stdin reading).
package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// sink collects writes into a string, like the TS `(s) => (out += s)`.
func sink(dst *string) func(string) {
	return func(s string) { *dst += s }
}

// TestRunPrintsPrettyJSON mirrors "run prints pretty JSON and returns 0".
func TestRunPrintsPrettyJSON(t *testing.T) {
	var out, errOut string
	code, err := run(`{"a":1,"b":[2,3]}`, sink(&out), sink(&errOut))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty", errOut)
	}
	want := "{\n  \"a\": 1,\n  \"b\": [\n    2,\n    3\n  ]\n}\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}

// TestRunReportsParseError mirrors "run reports parse errors on stderr
// and returns 1".
func TestRunReportsParseError(t *testing.T) {
	var out, errOut string
	code, err := run(`{a:1}`, sink(&out), sink(&errOut))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if out != "" {
		t.Fatalf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "unexpected") {
		t.Fatalf("stderr = %q, want it to mention %q", errOut, "unexpected")
	}
}

// TestMainParsesArgs mirrors "main parses command-line arguments".
func TestMainParsesArgs(t *testing.T) {
	var out string
	code := runMain([]string{`{"a":1}`}, strings.NewReader(""), sink(&out), func(string) {})
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if !strings.Contains(out, `"a": 1`) {
		t.Fatalf("stdout = %q, want it to contain %q", out, `"a": 1`)
	}
}

// TestMainJoinsArgs checks the argv.join(' ') behavior: multiple
// arguments are concatenated with a single space before parsing.
func TestMainJoinsArgs(t *testing.T) {
	var out string
	code := runMain([]string{"[1,", "2,", "3]"}, strings.NewReader(""), sink(&out), func(string) {})
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	var got []any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(got, []any{float64(1), float64(2), float64(3)}) {
		t.Fatalf("parsed %v, want [1 2 3]", got)
	}
}

// TestMainReadsStdin mirrors "main reads JSON from stdin".
func TestMainReadsStdin(t *testing.T) {
	var out string
	code := runMain(nil, strings.NewReader("[1,2,3]"), sink(&out), func(string) {})
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	var got []any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(got, []any{float64(1), float64(2), float64(3)}) {
		t.Fatalf("parsed %v, want [1 2 3]", got)
	}
}

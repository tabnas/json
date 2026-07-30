// Copyright (c) 2026 tabnas, MIT License

package tabnasjson

// parity_test.go — cross-runtime conformance, driven by the shared
// `test/spec/*.tsv` fixtures at the repo root (see ../test/AGENTS.md), the
// same convention @tabnas/parser and @tabnas/abnf use.
//
// ts/test/parity.test.js discovers and runs the SAME files, so the two
// implementations cannot drift without one of them going red. Every row is
// also cross-checked against encoding/json, since this package's contract is
// plain JSON.

import (
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tabnas "github.com/tabnas/parser/go"
)

type specRow struct {
	file     string
	lineNo   int
	input    string
	expected string
}

func specDir() string { return filepath.Join("..", "test", "spec") }

// specUnescape decodes the escape set used in the input column. Kept
// byte-identical to the TS loader so both runtimes feed the parser the exact
// same source text.
func specUnescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func loadSpec(t *testing.T, path string) []specRow {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("spec file not found: %s: %v", path, err)
	}
	var rows []specRow
	for i, line := range strings.Split(string(data), "\n") {
		// A comment line starts with '#' and has no tab; a data row always
		// has at least one (input + expected), so '#'-leading sources still
		// work.
		if i == 0 || line == "" || (strings.HasPrefix(line, "#") && !strings.Contains(line, "\t")) {
			continue // header, blank and comment lines
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 2 {
			t.Fatalf("%s:%d: expected at least 2 tab-separated columns", path, i+1)
		}
		rows = append(rows, specRow{
			file:     filepath.Base(path),
			lineNo:   i + 1,
			input:    specUnescape(cols[0]),
			expected: cols[1],
		})
	}
	if len(rows) == 0 {
		t.Fatalf("%s: no cases", path)
	}
	return rows
}

func runSpecFile(t *testing.T, path string) {
	for _, row := range loadSpec(t, path) {
		t.Run(row.input, func(t *testing.T) {
			got, err := Parse(row.input)

			if strings.HasPrefix(row.expected, "ERROR") {
				code := strings.TrimPrefix(strings.TrimPrefix(row.expected, "ERROR"), ":")
				if err == nil {
					t.Fatalf("%s:%d: expected error %q, got %v", row.file, row.lineNo, code, got)
				}
				je, ok := err.(*tabnas.TabnasError)
				if !ok {
					t.Fatalf("%s:%d: got %T, want *tabnas.TabnasError", row.file, row.lineNo, err)
				}
				// The error code is part of the shared parity contract: both
				// runtimes must reject the input with the same code.
				if je.Code != code {
					t.Fatalf("%s:%d: code = %q, want %q", row.file, row.lineNo, je.Code, code)
				}
				// Sanity: the standard library must reject it too.
				var sink any
				if stdjson.Unmarshal([]byte(row.input), &sink) == nil {
					t.Fatalf("%s:%d: encoding/json accepted %q", row.file, row.lineNo, row.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("%s:%d: Parse(%q) returned error: %v", row.file, row.lineNo, row.input, err)
			}

			// Cross-check against the standard library parser. DeepEqual
			// compares floats with ==, so -0 and 0 (both valid, both the
			// value zero) are treated as equal.
			var want any
			if err := stdjson.Unmarshal([]byte(row.input), &want); err != nil {
				t.Fatalf("%s:%d: encoding/json rejected valid fixture %q: %v",
					row.file, row.lineNo, row.input, err)
			}
			// Parsed objects are insertion-ordered *tabnas.OrderedMap; deorder
			// unwraps them to plain maps so the comparison is by value.
			// Key order is separately covered by TestSpecValidOrder.
			if !reflect.DeepEqual(deorder(got), want) {
				t.Fatalf("%s:%d: Parse(%q) = %s, want %s",
					row.file, row.lineNo, row.input, canon(t, got), canon(t, want))
			}
			// And the fixture's own expected JSON must agree. Compared by
			// value, not by text: the column is written with JS
			// JSON.stringify semantics, which renders -0 as "0".
			var fixture any
			if err := stdjson.Unmarshal([]byte(row.expected), &fixture); err != nil {
				t.Fatalf("%s:%d: bad expected JSON %q: %v",
					row.file, row.lineNo, row.expected, err)
			}
			if !reflect.DeepEqual(deorder(got), fixture) {
				t.Fatalf("%s:%d: Parse(%q) = %s, want %s",
					row.file, row.lineNo, row.input, canon(t, got), row.expected)
			}
		})
	}
}

// TestSpec auto-discovers every fixture: adding a .tsv runs it in both
// runtimes without touching either runner.
func TestSpec(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(specDir(), "*.tsv"))
	if err != nil {
		t.Fatalf("glob spec dir: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no spec files under %s", specDir())
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) { runSpecFile(t, path) })
	}
}

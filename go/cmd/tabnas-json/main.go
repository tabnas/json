// Copyright (c) 2026 tabnas, MIT License

// Command tabnas-json is a tiny command-line front end: read JSON from
// the arguments or from stdin, parse it (strict, standard JSON), and
// print the canonical re-serialized form. Exit code 1 (with the error
// on stderr) when the input is not valid JSON.
//
// It is the Go port of ts/src/json-cli.ts (driven by ts/bin/json). The
// logic is split into `run` (pure: source in, exit code out, writes via
// injected sinks) and `main`/`runMain` (process wiring), so both are
// testable in-process.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	tjson "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

// run parses src, writing the pretty-printed value to out on success or
// the error message to errOut on failure. Returns the process exit code.
//
// Mirrors json-cli.ts `run`: on success it emits JSON.stringify(value,
// null, 2) + "\n" (Go json.MarshalIndent with a 2-space indent matches
// that layout); on a *tabnas.TabnasError it writes the formatted message
// and returns 1. Any other (non-parse) error is returned to the caller,
// matching the TS `throw err` for non-TabnasError failures.
func run(src string, out, errOut func(string)) (int, error) {
	val, err := tjson.Parse(src)
	if err != nil {
		if je, ok := err.(*tabnas.TabnasError); ok {
			errOut(je.Error() + "\n")
			return 1, nil
		}
		return 0, err
	}
	b, merr := json.MarshalIndent(val, "", "  ")
	if merr != nil {
		return 0, merr
	}
	out(string(b) + "\n")
	return 0, nil
}

// runMain wires run to a process-like environment. With arguments, parse
// them (joined by a single space, matching argv.join(' ')); otherwise
// read all of stdin. Returns the resulting exit code.
//
// Mirrors json-cli.ts `main`, but returns the code rather than calling an
// injected exit, since Go's main calls os.Exit directly.
func runMain(args []string, stdin io.Reader, out, errOut func(string)) int {
	var src string
	if len(args) > 0 {
		src = join(args, " ")
	} else {
		b, err := io.ReadAll(stdin)
		if err != nil {
			errOut(err.Error() + "\n")
			return 1
		}
		src = string(b)
	}
	code, err := run(src, out, errOut)
	if err != nil {
		errOut(err.Error() + "\n")
		return 1
	}
	return code
}

// join concatenates parts with sep (a tiny strings.Join, kept local so
// the file's only stdlib value deps are io/os/json/fmt).
func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func main() {
	code := runMain(
		os.Args[1:],
		os.Stdin,
		func(s string) { fmt.Fprint(os.Stdout, s) },
		func(s string) { fmt.Fprint(os.Stderr, s) },
	)
	os.Exit(code)
}

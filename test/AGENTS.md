# Agents Guide — shared spec fixtures

`spec/*.tsv` holds the cross-runtime conformance fixtures. Both runtimes
auto-discover and run **every** file in this directory, so a change here
affects TypeScript and Go together — edit with that in mind.

These moved up from `ts/test/spec/` so the fixtures sit above both
runtimes rather than inside one of them, matching @tabnas/parser.

## Format

Tab-separated, one case per line, with a header row naming the columns.
Blank lines are skipped, and so are comment lines — a line starting with
`#` that contains no tab. (A data row always has at least one tab, so a
`#`-leading source such as a C preprocessor directive still works.)

| Column | Meaning |
|---|---|
| `input` | JSON source. Escapes `\n` `\r` `\t` `\\` are decoded. |
| `expected` | The parse result as JSON, or `ERROR:<code>` for input that must be rejected. |
| `opts` | Unused here — this package parses plain JSON with no options. |

`expected` is **not** escape-decoded — it is raw JSON, so JSON's own escape
rules apply. To put a literal backslash in `input`, write `\\`.

The `expected` column is written with JavaScript `JSON.stringify` semantics
(so `-0` renders as `0`); the Go runner compares by value rather than by
text, which keeps that difference from mattering.

The error **code** is part of the contract, not just "it threw": both
runtimes must reject an `ERROR:` row with exactly that code. Every row is
additionally cross-checked against the platform JSON parser — `JSON.parse`
in TypeScript, `encoding/json` in Go — because this package's whole contract
is plain JSON.

## Who runs what

- TypeScript: `ts/test/parity.test.js` — reads `../../test/spec`.
- Go: `go/parity_test.go` — `TestSpec` globs `../test/spec/*.tsv`.

Both discover files by directory listing: adding a `.tsv` here runs it in
both runtimes without touching either runner.

## Rules

- Prefer adding a fixture here over a one-off in-language assertion when a
  case is expressible as input → output. That is what keeps the two
  runtimes honest against each other.
- TypeScript is canonical. If the two runtimes disagree, the TS behaviour is
  the expected value — unless Go has exposed a genuine TS defect, in which
  case fix TS first and pin the corrected behaviour here.
- A new fixture must pass in BOTH runtimes: run `go test ./...` (from `go/`)
  and `npm test` (from `ts/`) before considering it done.

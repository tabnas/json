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

## The one exception: `rfc8259-gaps.tsv`

`spec/rfc8259-gaps.tsv` is **deliberately red** and is the single fixture
file exempt from the "must pass in BOTH runtimes" rule above. Every row is
input RFC 8259 requires a parser to reject, that `JSON.parse` and
`encoding/json` both reject, and that one of the two runtimes currently
accepts. It pins the CORRECT behaviour so the gap cannot be forgotten:

- 11 rows fail in **TypeScript** — trailing content after a complete
  top-level value is silently discarded (`"a" "b"` parses as `"a"`).
- 4 rows fail in **Go** — non-string object keys are accepted and coerced
  to the empty string (`{1:1}` parses as `{"":1}`).

Do not delete rows, relax the expected codes, rename the file out of the
auto-discovery glob, or move the cases somewhere that does not run, in
order to get green. The fix belongs in the parsers. When both runtimes
reject all of these, the file goes green on its own and the exception
disappears with it.

## The third-party corpus

`spec/*.tsv` is the *parity* contract between the two runtimes. It is not,
by itself, evidence of RFC conformance — a hand-written fixture set only
tests what its author thought to write down. The external check is
[nst/JSONTestSuite](https://github.com/nst/JSONTestSuite) (318 cases: 95
`y_` must-accept, 188 `n_` must-reject, 35 `i_` implementation-defined),
run by `ts/test/jsontestsuite.test.js` and `go/jsontestsuite_test.go`.

The corpus is **third-party and is never committed**. It is fetched at a
pinned commit into the gitignored `test/jsontestsuite/` by
`scripts/fetch-jsontestsuite.sh` — run automatically by the `pretest` npm
script (TS) and by `TestMain` (Go). Both runners assert the corpus is
intact (318 files, 95/188/35) before grading, so narrowing it goes red,
and both **fail loudly rather than skip** when it is absent: a conformance
test that quietly does not run reports a green tick that is a lie.

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

- TypeScript: `ts/test/parity.test.js` — `makeRunner(...).dir(...)`.
- Go: `go/parity_test.go` — `support.Runner{...}.Dir(t, dir)`.

Both are a dozen lines holding only what is specific to this package: the
cross-check against the platform parser, and (in Go) unwrapping the
insertion-ordered map. Everything else — finding `test/spec`, reading the
file, decoding escapes, the `ERROR:` contract, the comparison, the
`<file>:<line>` in a failure message — comes from
[`@tabnas/support`](https://github.com/tabnas/support) and its Go half, so
the two loaders cannot drift from each other either.

The Go cross-check for rejected input is its own walk,
`TestSpecRejectedByBoth`, because the runner hands its error hook an error
rather than the input that caused it.

Both discover files by directory listing: adding a `.tsv` here runs it in
both runtimes without touching either runner. An empty fixture, and a spec
directory with no fixtures in it, both **fail** — a runner that reports
green having run nothing is indistinguishable from coverage that was never
there.

## Rules

- Prefer adding a fixture here over a one-off in-language assertion when a
  case is expressible as input → output. That is what keeps the two
  runtimes honest against each other.
- TypeScript is canonical. If the two runtimes disagree, the TS behaviour is
  the expected value — unless Go has exposed a genuine TS defect, in which
  case fix TS first and pin the corrected behaviour here.
- A new fixture must pass in BOTH runtimes: run `go test ./...` (from `go/`)
  and `npm test` (from `ts/`) before considering it done.

## The external corpus

`spec/*.tsv` is the *parity* contract between the two runtimes. It is not,
by itself, evidence of RFC conformance — a hand-written fixture set only
tests what its author thought to write down. The external check is
[nst/JSONTestSuite](https://github.com/nst/JSONTestSuite) (318 cases: 95
`y_` must-accept, 188 `n_` must-reject, 35 `i_` implementation-defined),
graded by `ts/test/conformance.test.js` and `go/conformance_test.go`.

That corpus is third-party and is **never committed**. `fetch-jsontestsuite.sh`
clones it at a pinned commit into the gitignored `jsontestsuite/`, and both
runtimes run that fetch themselves — `pretest` in ts/, `TestMain` in go/ —
so it is present in CI as well as locally. Both runners assert the census
(95/188/35) before grading, and both **fail rather than skip** when the
corpus is absent: a conformance test that quietly does not run reports a
green tick that is a lie.

The two layers are complementary. When the corpus turns up a real defect,
fix it and then distil the case into a `spec/*.tsv` row, so the behaviour
stays pinned even without a network fetch — that is where the
trailing-content rows at the end of `errors.tsv` came from.

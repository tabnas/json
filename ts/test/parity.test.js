/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

// Cross-runtime conformance, driven by the shared `test/spec/*.tsv` fixtures
// at the repo root (see ../../test/AGENTS.md).
//
// The fixture loader, the escape codec, the `ERROR:<code>` contract and the
// row loop all come from @tabnas/support, whose Go half `go/parity_test.go`
// uses to run the SAME files — so the two implementations cannot drift
// without one of them going red, and neither can the two loaders.
//
// What is left here is only what is specific to @tabnas/json: every row is
// also cross-checked against the platform JSON parser, since this package's
// contract IS plain JSON.

const { findSpecDir, makeRunner } = require('@tabnas/support')

const { parse, TabnasError } = require('../dist/json.js')

// `JSON.stringify` with ONE correction: it renders negative zero as `0`,
// losing a distinction the value contract keeps (tabnas/support#13, and
// ADR-15). Go's `json.Marshal` renders it `-0`, so a fixture cell compared
// as text could not previously hold `-0` at all — the row was the only
// input in the suite where this runner and the Go one disagreed about what
// the `expected` column even means.
//
// Everything else defers to `JSON.stringify`, and objects are walked in
// `Object.keys` order, which is insertion order for string keys — so this
// still pins KEY ORDER, which is why the comparison is textual.
function render(value) {
  if ('number' === typeof value && Object.is(value, -0)) return '-0'
  if (Array.isArray(value)) return '[' + value.map(render).join(',') + ']'
  if (null !== value && 'object' === typeof value) {
    return '{' + Object.keys(value)
      .map((k) => JSON.stringify(k) + ':' + render(value[k])).join(',') + '}'
  }
  return JSON.stringify(value)
}

makeRunner({
  parse: (input) => {
    const value = parse(input)
    const text = render(value)

    // The platform parser must accept it, and produce the same thing. A
    // throw here surfaces as a failed row, naming the fixture and line.
    // Rendered the same way, so the oracle is held to the same contract
    // rather than being allowed to lose the sign where we do not.
    const platform = render(JSON.parse(input))
    if (text !== platform) {
      throw new Error(
        `disagrees with the platform parser: ${text} != ${platform}`)
    }

    return text
  },

  // Compared as TEXT, not structurally: the expected column is written
  // with `JSON.stringify` semantics (as corrected by `render` above), and
  // comparing the rendering is what pins KEY ORDER as well as value. (Go
  // pins order separately, in TestSpecValidOrder — it compares by value
  // here.)
  parseExpected: (expected) => expected,

  // Two sanity checks the code comparison would not otherwise make. Each
  // answers a pseudo-code rather than throwing, so the failure reads as
  // "failed with code <what went wrong>, expected <the row's code>" and
  // names the row.
  errorCode: (err, row) => {
    if (!(err instanceof TabnasError)) {
      return `not-a-TabnasError(${err})`
    }
    try {
      JSON.parse(row.unesc(0))
      return 'the-platform-parser-accepted-it'
    }
    catch {
      // Rejected by both, as it must be.
    }
    return err.code
  },
})
  // `findSpecDir` walks up from this file to the repo root's `test/spec`,
  // so moving the suite does not mean recounting `..` hops. `dir` then
  // auto-discovers every fixture in it, so adding a .tsv runs it in both
  // runtimes without touching either runner.
  .dir(findSpecDir(__dirname))

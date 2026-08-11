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

makeRunner({
  parse: (input) => {
    const value = parse(input)
    const text = JSON.stringify(value)

    // The platform parser must accept it, and produce the same thing. A
    // throw here surfaces as a failed row, naming the fixture and line.
    const platform = JSON.stringify(JSON.parse(input))
    if (text !== platform) {
      throw new Error(
        `disagrees with the platform parser: ${text} != ${platform}`)
    }

    return text
  },

  // Compared as TEXT, not structurally: the expected column is written
  // with `JSON.stringify` semantics, and comparing the rendering is what
  // pins KEY ORDER as well as value. (Go pins order separately, in
  // TestSpecValidOrder — it compares by value here.)
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

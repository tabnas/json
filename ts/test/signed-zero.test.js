/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */
'use strict'

// Negative zero, pinned per port because the shared fixture cannot hold it.
//
// `-0` used to be a row in test/spec/valid.tsv. It cannot stay there, and
// the reason is worth writing down rather than rediscovering.
//
// The two halves of that fixture ask DIFFERENT QUESTIONS of the `expected`
// column. This runner (test/parity.test.js) compares the RENDERING —
// `JSON.stringify(parse(input))` against the cell — which is deliberate,
// because comparing text pins key ORDER as well as value. The Go runner
// compares the VALUE, and pins order separately in TestSpecValidOrder.
//
// For every other input those two questions have the same answer. For
// negative zero they do not, because the renderings differ:
//
//     JS    JSON.stringify(-0)      "0"     — the sign is lost
//     Go    json.Marshal(-0)        "-0"    — the sign survives
//
// So a text-compared cell must say `0` here and `-0` there, and one column
// cannot be both. The row was green until tabnas/support#13 put signed
// zero in the value contract, at which point Go's value comparison started
// distinguishing `0` from `-0` and the cell became wrong for Go while
// staying right here.
//
// The VALUE fact is not in dispute — both ports parse `-0` to negative
// zero — so it is pinned here and in go/signed_zero_test.go instead, where
// each port can assert it in terms its own runtime can express.
// `Object.is` is the only way to say it in JavaScript: `-0 === 0` is true.
// ADR-15: signed zero is in the value contract.

const { describe, it } = require('node:test')
const assert = require('node:assert')

const { parse } = require('../dist/json.js')

describe('signed zero', () => {
  it('parses -0 as negative zero, and 0 as positive zero', () => {
    const cases = [
      // The subject.
      ['-0', -0],

      // Controls. Without them "distinguishes signed zero" is also
      // satisfied by reporting every zero as negative, or by never looking
      // at the sign at all.
      ['0', 0],
      ['-0.0', -0],
      ['0.0', 0],
      ['-1', -1],
    ]
    for (const [src, want] of cases) {
      const got = parse(src)
      assert.ok(Object.is(got, want),
        `${src}: parsed as ${Object.is(got, -0) ? '-0' : String(got)}, ` +
        `want ${Object.is(want, -0) ? '-0' : String(want)}`)
    }
  })

  it('renders negative zero as 0, which is why the shared cell cannot hold it', () => {
    // Not incidental — this is the whole reason the row moved out of
    // valid.tsv. If JSON.stringify ever stopped losing the sign, the row
    // could go back and this file could go.
    assert.equal(JSON.stringify(parse('-0')), '0')
    assert.ok(Object.is(parse('-0'), -0))
  })
})

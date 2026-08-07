/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

// Third-party RFC 8259 / ECMA-404 conformance: nst/JSONTestSuite, the corpus
// behind "Parsing JSON is a Minefield" (http://seriot.ch/projects/parsing_json.html).
//
// The corpus is NOT vendored. `scripts/fetch-jsontestsuite.sh` clones it at a
// pinned commit into the gitignored `test/jsontestsuite/`, and the `pretest`
// npm script runs that fetch before the suite. `go/jsontestsuite_test.go`
// grades the SAME files, so the two runtimes can be compared case for case.
//
// This suite MUST NOT be able to skip. If the corpus is absent, loading this
// file throws and the whole test file fails with instructions -- a
// conformance test that quietly does not run is worse than no test at all,
// because the green tick is a lie.
//
// Grading (see the upstream README):
//   y_  MUST be accepted   -- and must produce the CORRECT VALUE, not merely
//                             "it did not throw". The oracle is the platform
//                             parser, V8's JSON.parse: independent of this
//                             package, so the assertion is not circular.
//   n_  MUST be rejected    -- with a TabnasError.
//   i_  implementation-defined by RFC 8259. This package's declared bar is
//       parity with the platform parser (README / AGENTS.md rule 4: "If
//       JSON.parse (TS/JS) or encoding/json (Go) would reject the input, so
//       does this parser"), so i_ is graded against JSON.parse -- same
//       accept/reject decision, and the same value when accepted.

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const { parse, TabnasError } = require('../dist/json.js')

const CORPUS = Path.join(__dirname, '..', '..', 'test', 'jsontestsuite')
const PARSING = Path.join(CORPUS, 'test_parsing')

// Exact shape of the corpus at the pinned commit
// (1ef36fa01286573e846ac449e8683f8833c5b26a). Re-asserted below so that
// quietly narrowing the corpus goes red instead of inflating the pass rate.
const EXPECT = { y: 95, n: 188, i: 35, total: 318 }

const MISSING =
  'nst/JSONTestSuite corpus not found at ' +
  PARSING +
  '\n\nThe conformance corpus is third-party and is deliberately NOT vendored.' +
  '\nFetch it (pinned commit, idempotent) and re-run:' +
  '\n\n    scripts/fetch-jsontestsuite.sh\n\n' +
  'This test FAILS rather than skips on purpose: a conformance suite that ' +
  'quietly does not run reports a green tick that is a lie.'

if (!Fs.existsSync(PARSING)) {
  throw new Error(MISSING)
}

const files = Fs.readdirSync(PARSING)
  .filter((f) => f.endsWith('.json'))
  .sort()

const group = (p) => files.filter((f) => f.startsWith(p))
const yFiles = group('y_')
const nFiles = group('n_')
const iFiles = group('i_')

// A JS string cannot hold invalid UTF-8, so the bytes are decoded the way any
// real JavaScript caller receives them: UTF-8 with U+FFFD replacement. Go can
// and does feed the raw bytes, which is a genuine and reported TS/Go
// difference on the handful of n_string_*_utf8 cases, not a fudge here.
const source = (f) => Fs.readFileSync(Path.join(PARSING, f)).toString('utf8')

const stable = (v) => JSON.stringify(v)

describe('jsontestsuite (nst/JSONTestSuite, RFC 8259)', () => {
  it('corpus is intact at the pinned commit', () => {
    assert.deepStrictEqual(
      { y: yFiles.length, n: nFiles.length, i: iFiles.length, total: files.length },
      EXPECT,
      'corpus has been narrowed or replaced -- re-run scripts/fetch-jsontestsuite.sh',
    )
  })

  describe('y_ must be accepted AND produce the correct value', () => {
    for (const f of yFiles) {
      it(f, () => {
        const src = source(f)
        // The oracle. If this throws, the corpus or the decode is wrong,
        // not the parser -- and that is worth failing loudly for too.
        const want = JSON.parse(src)
        const got = parse(src)
        assert.strictEqual(
          stable(got),
          stable(want),
          `${f}: value differs from JSON.parse`,
        )
      })
    }
  })

  describe('n_ must be rejected', () => {
    for (const f of nFiles) {
      it(f, () => {
        const src = source(f)
        let got
        let threw = false
        try {
          got = parse(src)
        } catch (e) {
          threw = true
          assert.ok(
            e instanceof TabnasError,
            `${f}: threw ${e && e.constructor && e.constructor.name}, want TabnasError`,
          )
          assert.ok(
            'string' === typeof e.code && '' !== e.code,
            `${f}: TabnasError has no code`,
          )
        }
        assert.ok(
          threw,
          `${f}: accepted input RFC 8259 requires a parser to reject; ` +
            `parsed as ${stable(got)}`,
        )
      })
    }
  })

  // Implementation-defined by the RFC, but NOT by this package: its declared
  // contract is platform parity, so JSON.parse decides.
  describe('i_ must match the platform parser (JSON.parse)', () => {
    for (const f of iFiles) {
      it(f, () => {
        const src = source(f)
        let want
        let platformOk = true
        try {
          want = JSON.parse(src)
        } catch {
          platformOk = false
        }

        let got
        let ok = true
        try {
          got = parse(src)
        } catch {
          ok = false
        }

        assert.strictEqual(
          ok,
          platformOk,
          `${f}: this parser ${ok ? 'accepted' : 'rejected'} but JSON.parse ` +
            `${platformOk ? 'accepted' : 'rejected'}`,
        )
        if (platformOk) {
          assert.strictEqual(
            stable(got),
            stable(want),
            `${f}: value differs from JSON.parse`,
          )
        }
      })
    }
  })
})

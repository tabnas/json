/* Copyright (c) 2026 tabnas, MIT License */

// conformance.test.js — the external RFC 8259 conformance suite.
//
// Runs nst/JSONTestSuite (https://github.com/nst/JSONTestSuite), the
// standard cross-implementation JSON parsing suite, against this package.
//
// The suite is not vendored; it is fetched at a pinned commit into the
// .gitignore'd test/jsontestsuite/ by test/fetch-jsontestsuite.sh, which the
// `pretest` npm script runs before this file — so `npm test`, locally and in
// CI, always grades against it.
//
// If the corpus is absent this file THROWS rather than skips. A conformance
// suite that quietly does not run reports a green tick that is a lie: it
// says "RFC 8259 conformant" while measuring nothing.
//
// The suite's file-name prefixes are the contract:
//   y_  MUST be accepted by a conforming parser — and must produce the same
//       VALUE as the platform parser, not merely "it did not throw". The
//       oracle, JSON.parse, is independent of this package, so the
//       assertion is not circular.
//   n_  MUST be rejected by a conforming parser, with an error code.
//   i_  implementation-defined; this package's contract is parity with the
//       platform parser, so we assert we agree with JSON.parse.
//
// go/conformance_test.go runs the SAME directory with the same rules, so
// the two runtimes cannot drift on it.

const { test, describe } = require('node:test')
const Assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const { parse } = require('../dist/json.js')

const SUITE = Path.join(__dirname, '..', '..', 'test', 'jsontestsuite', 'test_parsing')

// Exact shape of the corpus at the pinned commit
// (1ef36fa01286573e846ac449e8683f8833c5b26a). Asserted before grading, so
// narrowing the corpus goes red instead of inflating the pass rate.
const EXPECT = { y: 95, n: 188, i: 35, total: 318 }

if (!Fs.existsSync(SUITE)) {
  throw new Error(
    'nst/JSONTestSuite corpus not found at ' + SUITE +
      '\n\nIt is third-party and deliberately not vendored. Fetch it' +
      '\n(pinned commit, idempotent) and re-run:' +
      '\n\n    sh test/fetch-jsontestsuite.sh    # or: make json-test-suite\n\n' +
      'This fails rather than skips on purpose: a conformance suite that ' +
      'quietly does not run reports a green tick that is a lie.',
  )
}

const files = Fs.readdirSync(SUITE).filter((f) => f.endsWith('.json')).sort()
const group = (p) => files.filter((f) => f.startsWith(p))

const stable = (v) => JSON.stringify(v)

describe('conformance: JSONTestSuite (RFC 8259)', () => {
  test('the corpus is intact at the pinned commit', () => {
    Assert.deepStrictEqual(
      {
        y: group('y_').length,
        n: group('n_').length,
        i: group('i_').length,
        total: files.length,
      },
      EXPECT,
      `corpus has been narrowed or replaced (under ${SUITE}) — ` +
        're-run `sh test/fetch-jsontestsuite.sh --force`',
    )
  })

  test('y_* : accepted, with the same value as JSON.parse', () => {
    const bad = []
    for (const f of group('y_')) {
      const src = Fs.readFileSync(Path.join(SUITE, f), 'utf8')
      // The oracle. If this throws, the corpus or the decode is wrong
      // rather than the parser — worth failing loudly for either way.
      Assert.doesNotThrow(() => JSON.parse(src), `JSON.parse rejected ${f}`)
      const want = JSON.parse(src)
      let got
      try {
        got = parse(src)
      } catch (e) {
        bad.push(`${f}: rejected (${e.code || e.message})`)
        continue
      }
      if (stable(got) !== stable(want)) {
        bad.push(`${f}: ${stable(got)} != ${stable(want)} (JSON.parse)`)
      }
    }
    Assert.deepEqual(bad, [], 'must-accept cases were rejected or misparsed')
  })

  test('n_* : rejected', () => {
    const bad = []
    for (const f of group('n_')) {
      const src = Fs.readFileSync(Path.join(SUITE, f), 'utf8')
      let accepted = false
      try {
        parse(src)
        accepted = true
      } catch (e) {
        // Every rejection must carry one of the contract error codes.
        Assert.ok(e.code, `${f}: rejected without an error code`)
      }
      if (accepted) bad.push(f)
      // Sanity: the platform parser must reject it too.
      Assert.throws(() => JSON.parse(src), `JSON.parse accepted ${f}`)
    }
    Assert.deepEqual(bad, [], 'must-reject cases were accepted')
  })

  test('i_* : same accept/reject and same value as JSON.parse', () => {
    const bad = []
    for (const f of group('i_')) {
      const src = Fs.readFileSync(Path.join(SUITE, f), 'utf8')
      let ours, ourOk = true
      try {
        ours = parse(src)
      } catch (e) {
        ourOk = false
      }
      let plat, platOk = true
      try {
        plat = JSON.parse(src)
      } catch (e) {
        platOk = false
      }
      if (ourOk !== platOk) {
        bad.push(`${f}: ours=${ourOk ? 'accept' : 'reject'} JSON.parse=${platOk ? 'accept' : 'reject'}`)
      } else if (ourOk && stable(ours) !== stable(plat)) {
        bad.push(`${f}: ${stable(ours)} != ${stable(plat)}`)
      }
    }
    Assert.deepEqual(bad, [], 'implementation-defined cases diverged from JSON.parse')
  })
})

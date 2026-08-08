/* Copyright (c) 2026 tabnas, MIT License */

// conformance.test.js — the external RFC 8259 conformance suite.
//
// Runs nst/JSONTestSuite (https://github.com/nst/JSONTestSuite), the
// standard cross-implementation JSON parsing suite, against this package.
// The suite is not vendored; fetch it with `sh test/fetch-jsontestsuite.sh`
// (or `make json-test-suite`) and these tests light up. Without it they
// skip, and the RFC 8259 claim is unverified.
//
// The suite's file-name prefixes are the contract:
//   y_  MUST be accepted by a conforming parser
//   n_  MUST be rejected by a conforming parser
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

// The suite has 318 cases today; a floor well under that still catches a
// half-cloned or empty directory passing vacuously.
const MIN_CASES = 300

const present = Fs.existsSync(SUITE)
const files = present ? Fs.readdirSync(SUITE).sort() : []

describe('conformance: JSONTestSuite (RFC 8259)', { skip: present ? false : 'JSONTestSuite not fetched — run `make json-test-suite`' }, () => {
  test('the suite is fully present', () => {
    Assert.ok(
      files.length >= MIN_CASES,
      `found ${files.length} cases under ${SUITE}, expected >= ${MIN_CASES}`
    )
  })

  test('y_* : accepted', () => {
    const bad = []
    for (const f of files.filter((f) => f.startsWith('y_'))) {
      const src = Fs.readFileSync(Path.join(SUITE, f), 'utf8')
      try {
        parse(src)
      } catch (e) {
        bad.push(`${f}: ${e.code || e.message}`)
      }
      // Sanity: the platform parser must accept it too.
      Assert.doesNotThrow(() => JSON.parse(src), `JSON.parse rejected ${f}`)
    }
    Assert.deepEqual(bad, [], 'must-accept cases were rejected')
  })

  test('n_* : rejected', () => {
    const bad = []
    for (const f of files.filter((f) => f.startsWith('n_'))) {
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
    for (const f of files.filter((f) => f.startsWith('i_'))) {
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
      } else if (ourOk && JSON.stringify(ours) !== JSON.stringify(plat)) {
        bad.push(`${f}: ${JSON.stringify(ours)} != ${JSON.stringify(plat)}`)
      }
    }
    Assert.deepEqual(bad, [], 'implementation-defined cases diverged from JSON.parse')
  })
})

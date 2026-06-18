/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')

const { parse, make } = require('../dist/json.js')

// Guards against a performance regression where the convenience `parse`
// rebuilds the (expensive) JSON grammar on every call instead of reusing a
// cached instance. Building the engine + grammar dominates a parse, so a
// rebuild-per-call `parse` is many times slower than reusing one `make()`
// instance.
//
// The check is machine-INDEPENDENT: it compares `parse` against instance
// reuse on the SAME machine in the SAME run, so a slow CI box cannot make it
// flaky (both sides scale together). There is deliberately NO wall-clock
// budget.
describe('perf', () => {
  it('parse() reuses a cached instance (no per-call grammar rebuild)', () => {
    const src = '{"a":1,"b":2,"c":[3,4,5]}'
    const n = 5000

    // Warm both paths so the comparison is steady-state.
    for (let i = 0; i < 200; i++) parse(src)
    const tn = make()
    for (let i = 0; i < 200; i++) tn.parse(src)

    const t0 = process.hrtime.bigint()
    for (let i = 0; i < n; i++) parse(src)
    const conv = Number(process.hrtime.bigint() - t0)

    const t1 = process.hrtime.bigint()
    for (let i = 0; i < n; i++) tn.parse(src)
    const reuse = Number(process.hrtime.bigint() - t1)

    const ratio = conv / reuse
    // A cached parse() is ~= instance reuse; allow 4x for scheduling noise.
    // A rebuild-per-call parse() is many times slower here, so this catches
    // the regression without depending on absolute wall-clock speed.
    assert.ok(
      conv <= 4 * reuse,
      `parse() appears to rebuild the grammar on every call: ${n} parse() ` +
        `calls took ${conv}ns vs ${reuse}ns reusing one instance ` +
        `(ratio ${ratio.toFixed(1)}x, limit 4x). Cache a lazy default ` +
        `instance (see parse / defaultParser).`,
    )
    console.log(`parse()=${conv}ns reuse=${reuse}ns ratio=${ratio.toFixed(2)}x`)
  })
})

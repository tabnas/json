/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')

const { parse, TabnasError } = require('../dist/json.js')
const { loadTSV } = require('./utility')

// Valid rows assert that re-serializing the parsed value matches the
// expected JSON (and the platform JSON.parse).
function runValid(name) {
  for (const { row, cols } of loadTSV(name)) {
    const [input, expected] = cols
    it(`${name} row ${row}: ${input}`, () => {
      const value = parse(input)
      assert.strictEqual(JSON.stringify(value), expected, `input: ${input}`)
      assert.strictEqual(
        JSON.stringify(value),
        JSON.stringify(JSON.parse(input)),
        `input: ${input}`,
      )
    })
  }
}

// Error rows assert that parsing throws a TabnasError with the exact
// `code`. The code is part of the shared parity contract: both runtimes
// must reject the input with the same code (see AGENTS.md).
function runErrors(name) {
  for (const { row, cols } of loadTSV(name)) {
    const [input, code] = cols
    it(`${name} row ${row}: ${input} -> ${code}`, () => {
      assert.throws(
        () => parse(input),
        (err) => {
          assert.ok(err instanceof TabnasError, `expected TabnasError, got ${err}`)
          assert.strictEqual(err.code, code, `input: ${input}`)
          return true
        },
      )
      // The platform parser must also reject it.
      assert.throws(() => JSON.parse(input), `JSON.parse accepted: ${input}`)
    })
  }
}

describe('spec: json-valid', () => runValid('json-valid'))
describe('spec: json-errors', () => runErrors('json-errors'))

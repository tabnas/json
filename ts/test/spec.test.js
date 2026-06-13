/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')

const { parse, JsonError } = require('../dist/json.js')
const { loadTSV } = require('./utility')

// Run a shared .tsv fixture. Valid rows assert that re-serializing the
// parsed value matches the expected JSON; error rows assert that a
// JsonError with the expected code is thrown.
function runSpec(name) {
  for (const { row, cols } of loadTSV(name)) {
    const [input, expected] = cols
    if (expected.startsWith('ERROR:')) {
      const code = expected.slice('ERROR:'.length)
      it(`${name} row ${row}: ${input} -> ${expected}`, () => {
        assert.throws(
          () => parse(input),
          (err) => {
            assert.ok(err instanceof JsonError, `expected JsonError, got ${err}`)
            assert.strictEqual(err.code, code, `input: ${input}`)
            return true
          },
        )
      })
    } else {
      it(`${name} row ${row}: ${input}`, () => {
        const value = parse(input)
        assert.strictEqual(JSON.stringify(value), expected, `input: ${input}`)
        // Cross-check against the platform JSON.parse for valid cases.
        assert.deepStrictEqual(value, JSON.parse(input), `input: ${input}`)
      })
    }
  }
}

describe('spec: json-valid', () => runSpec('json-valid'))
describe('spec: json-errors', () => runSpec('json-errors'))

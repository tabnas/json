/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')

const { parse, TabnasError } = require('../dist/json.js')
const { loadTSV } = require('./utility')

// Run a shared .tsv fixture. Valid rows assert that re-serializing the
// parsed value matches the expected JSON (and the platform JSON.parse);
// error rows assert that parsing throws. Error codes are engine-specific
// and differ across runtimes (see AGENTS.md), so the shared contract is
// "rejects this input", not a particular code.
function runSpec(name) {
  for (const { row, cols } of loadTSV(name)) {
    const [input, expected] = cols
    if (expected === 'ERROR') {
      it(`${name} row ${row}: ${input} -> ERROR`, () => {
        assert.throws(
          () => parse(input),
          (err) => {
            assert.ok(
              err instanceof TabnasError,
              `expected TabnasError, got ${err}`,
            )
            return true
          },
        )
        // The platform parser must also reject it.
        assert.throws(() => JSON.parse(input), `JSON.parse accepted: ${input}`)
      })
    } else {
      it(`${name} row ${row}: ${input}`, () => {
        const value = parse(input)
        assert.strictEqual(JSON.stringify(value), expected, `input: ${input}`)
        // Cross-check against the platform JSON.parse.
        assert.strictEqual(
          JSON.stringify(value),
          JSON.stringify(JSON.parse(input)),
          `input: ${input}`,
        )
      })
    }
  }
}

describe('spec: json-valid', () => runSpec('json-valid'))
describe('spec: json-errors', () => runSpec('json-errors'))

/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

// Cross-runtime conformance, driven by the shared `test/spec/*.tsv` fixtures
// at the repo root — the same convention @tabnas/parser and @tabnas/abnf use
// (see ../../test/AGENTS.md).
//
// `go/parity_test.go` discovers and runs the SAME files, so the two
// implementations cannot drift without one of them going red. Every row is
// also cross-checked against the platform JSON parser, since this package's
// contract is plain JSON.

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const { parse, TabnasError } = require('../dist/json.js')

const specDir = Path.join(__dirname, '..', '..', 'test', 'spec')

// Decode the escape set used in the input column. Kept byte-identical to the
// Go loader so both runtimes feed the parser the exact same source text.
function unescape(s) {
  if (!s.includes('\\')) return s
  let out = ''
  for (let i = 0; i < s.length; i++) {
    const c = s[i]
    if ('\\' === c && i + 1 < s.length) {
      const n = s[i + 1]
      if ('n' === n) { out += '\n'; i++; continue }
      if ('r' === n) { out += '\r'; i++; continue }
      if ('t' === n) { out += '\t'; i++; continue }
      if ('\\' === n) { out += '\\'; i++; continue }
    }
    out += c
  }
  return out
}

function loadSpec(file) {
  const body = Fs.readFileSync(Path.join(specDir, file), 'utf8')
  const lines = body.split(/\r?\n/)
  const rows = []
  // Line 1 is the header naming the columns.
  for (let i = 1; i < lines.length; i++) {
    const raw = lines[i]
    // A comment line starts with '#' and has no tab; a data row always has
    // at least one (input + expected), so '#'-leading sources still work.
    if ('' === raw || (raw.startsWith('#') && !raw.includes('\t'))) continue
    const cols = raw.split('\t')
    if (cols.length < 2) {
      throw new Error(`${file}:${i + 1}: expected at least 2 tab-separated columns`)
    }
    rows.push({ line: i + 1, input: unescape(cols[0]), expected: cols[1] })
  }
  return rows
}

function runSpec(file) {
  const rows = loadSpec(file)
  describe('spec: ' + file, () => {
    assert.ok(0 < rows.length, file + ': no cases')
    for (const row of rows) {
      it(`row ${row.line}: ${row.input}`, () => {
        if (row.expected.startsWith('ERROR')) {
          const code = row.expected.slice('ERROR'.length).replace(/^:/, '')
          assert.throws(
            () => parse(row.input),
            (err) => {
              assert.ok(err instanceof TabnasError,
                `expected TabnasError, got ${err}`)
              // The error code is part of the shared parity contract: both
              // runtimes must reject the input with the same code.
              assert.strictEqual(err.code, code, `input: ${row.input}`)
              return true
            },
          )
          // Sanity: the platform parser must reject it too.
          assert.throws(() => JSON.parse(row.input),
            `JSON.parse accepted: ${row.input}`)
          return
        }

        const value = parse(row.input)
        assert.strictEqual(JSON.stringify(value), row.expected,
          `${file}:${row.line}`)
        assert.strictEqual(
          JSON.stringify(value),
          JSON.stringify(JSON.parse(row.input)),
          `${file}:${row.line}: disagrees with the platform parser`,
        )
      })
    }
  })
}

// Auto-discover every fixture: adding a .tsv runs it in both runtimes
// without touching either runner.
for (const file of Fs.readdirSync(specDir).sort()) {
  if (file.endsWith('.tsv')) runSpec(file)
}

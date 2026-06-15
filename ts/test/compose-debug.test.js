/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

// Optional composition test: the standard JSON grammar plugin layered
// with the official @tabnas/debug plugin. @tabnas/debug is NOT a declared
// dependency (it is an external, optional tool), so this test resolves it
// dynamically and SKIPS when it is absent — normal `npm test` stays
// self-contained. The `compose-debug` CI job builds @tabnas/debug from a
// sibling checkout and points TABNAS_DEBUG_PATH at its built plugin.

const { describe, it } = require('node:test')
const assert = require('node:assert')

const { Tabnas } = require('@tabnas/parser')
const { json } = require('../dist/json.js')

function loadDebug() {
  const candidates = [process.env.TABNAS_DEBUG_PATH, '@tabnas/debug'].filter(
    Boolean,
  )
  for (const c of candidates) {
    try {
      return require(c).Debug
    } catch {
      /* try next */
    }
  }
  return null
}

const Debug = loadDebug()
const skip = Debug ? false : '@tabnas/debug not available (set TABNAS_DEBUG_PATH)'

describe('compose: json + @tabnas/debug', () => {
  it('parses normally with the debug plugin installed', { skip }, () => {
    const tn = new Tabnas({ plugins: [json] })
    tn.use(Debug, { print: false, trace: false })
    assert.deepStrictEqual(
      JSON.parse(JSON.stringify(tn.parse('{"a":[1,2]}'))),
      { a: [1, 2] },
    )
  })

  it('debug.describe() introspects the JSON grammar', { skip }, () => {
    const tn = new Tabnas({ plugins: [json] })
    tn.use(Debug, { print: false, trace: false })
    const desc = tn.debug.describe()
    // The shared val / map / list / pair / elem rules are present.
    for (const rule of ['val', 'map', 'list', 'pair', 'elem']) {
      assert.match(desc, new RegExp('\\b' + rule + '\\b'), `missing rule: ${rule}`)
    }
  })
})

/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

// Composition test: the standard JSON grammar plugin layered with the
// official @tabnas/debug plugin. @tabnas/debug is a devDependency, but this
// still resolves it dynamically and SKIPS when it is absent so the suite
// stays runnable outside the package; the `compose-debug` CI job can also
// point TABNAS_DEBUG_PATH at a sibling checkout's built plugin.

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

  it('debug.model() returns the structured JSON grammar', { skip }, () => {
    const tn = new Tabnas({ plugins: [json] })
    tn.use(Debug, { print: false, trace: false })
    const m = tn.debug.model()

    // The structured rule set and entry rule.
    assert.deepStrictEqual(
      m.rules.map((r) => r.name).sort(),
      ['elem', 'list', 'map', 'pair', 'val'],
    )
    assert.equal(m.config.start, 'val')
    assert.ok(m.plugins.some((p) => p.name === 'json'), 'plugins should list json')

    // val is a choice whose open alts push the map and list rules.
    const val = m.rules.find((r) => r.name === 'val')
    assert.ok(val.open.some((a) => a.push === 'map'), 'val should push map')
    assert.ok(val.open.some((a) => a.push === 'list'), 'val should push list')

    // The grammar portion is JSON-serialisable and round-trips.
    const grammar = {
      tokens: m.tokens, rules: m.rules, graph: m.graph, config: m.config, abnf: m.abnf,
    }
    assert.deepStrictEqual(JSON.parse(JSON.stringify(grammar)).rules, m.rules)
  })
})

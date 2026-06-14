/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')

const Json = require('../dist/json.js')
const { parse, make, json, registerJsonGrammar, Tabnas, TabnasError, Version } =
  Json

// Normalize null-prototype engine objects for deepEqual comparison.
const norm = (v) => JSON.parse(JSON.stringify(v))

describe('json', () => {
  it('default export is parse', () => {
    assert.strictEqual(Json.default, parse)
  })

  it('exposes the plugin and grammar installer', () => {
    assert.strictEqual(typeof json, 'function')
    assert.strictEqual(typeof registerJsonGrammar, 'function')
    assert.strictEqual(typeof make, 'function')
  })

  it('exports a Version string', () => {
    assert.strictEqual(typeof Version, 'string')
    assert.match(Version, /^\d+\.\d+\.\d+$/)
  })

  it('make(opts) applies extra options after the grammar', () => {
    const am = make({ info: { map: true } })
    const out = am.parse('{"a":1}')
    const mark = Object.getOwnPropertyDescriptor(out, '__info__')
    assert.strictEqual(mark.value.implicit, false)
  })

  it('json is a usable tabnas plugin', () => {
    const am = new Tabnas({ plugins: [json] })
    assert.deepStrictEqual(norm(am.parse('{"a":[1,2,3]}')), { a: [1, 2, 3] })
  })

  it('supports the engine info options (foundation for other parsers)', () => {
    // Enabling info.map/list/text exercises the marker-attaching branches
    // the plain-JSON config leaves off; kept so other plugins can build on
    // this grammar.
    const am = new Tabnas({
      plugins: [json],
      info: { map: true, list: true, text: true },
    })
    const out = am.parse('{"a":["x",1],"__info__":9}')
    // The key matching the marker is dropped to preserve metadata.
    assert.deepStrictEqual(Object.keys(out), ['a'])
    const mapMark = Object.getOwnPropertyDescriptor(out, '__info__')
    assert.strictEqual(mapMark.value.implicit, false)
    // List marker.
    assert.ok(Array.isArray(out.a))
    const listMark = Object.getOwnPropertyDescriptor(out.a, '__info__')
    assert.strictEqual(listMark.value.implicit, false)
    // String values are wrapped with quote info.
    assert.ok(out.a[0] instanceof String)
    assert.strictEqual(String(out.a[0]), 'x')
    const textMark = Object.getOwnPropertyDescriptor(out.a[0], '__info__')
    assert.strictEqual(textMark.value.quote, '"')
  })

  it('parses scalars', () => {
    assert.strictEqual(parse('42'), 42)
    assert.strictEqual(parse('-3.14'), -3.14)
    assert.strictEqual(parse('"x"'), 'x')
    assert.strictEqual(parse('true'), true)
    assert.strictEqual(parse('false'), false)
    assert.strictEqual(parse('null'), null)
  })

  it('parses nested structures', () => {
    assert.deepStrictEqual(norm(parse('{"a":[1,{"b":2}]}')), { a: [1, { b: 2 }] })
  })

  it('handles surrogate pairs', () => {
    assert.strictEqual(parse('"\\uD83D\\uDE00"'), '😀')
  })

  it('handles all simple escapes', () => {
    assert.strictEqual(parse('"\\b\\f\\n\\r\\t\\/\\\\\\""'), '\b\f\n\r\t/\\"')
  })

  it('rejects empty and whitespace-only input', () => {
    assert.throws(() => parse(''), (e) => e instanceof TabnasError)
    assert.throws(() => parse('   '), (e) => e instanceof TabnasError)
  })

  it('reports an unterminated string', () => {
    assert.throws(
      () => parse('"abc'),
      (e) => e instanceof TabnasError && e.code === 'unterminated_string',
    )
  })

  it('rejects extended grammar that jsonic would accept', () => {
    for (const bad of [
      '{a:1}', // unquoted key
      '[1,2,]', // trailing comma
      '1 // note', // comment
      "'x'", // single quotes
      'a:1,b:2', // implicit object
      'x,y,z', // implicit array
      '0x10', // hex number
      '.5', // bare leading dot
      '+1', // leading plus
      '1.', // trailing dot
      '01', // leading zero
      '"\\x41"', // \xHH ascii escape
      '"\\u{41}"', // \u{...} braced escape
      '"\\v"', // non-standard \v escape
      '"\\\'"', // non-standard \' escape
      '"\\`"', // non-standard backtick escape
    ]) {
      assert.throws(() => parse(bad), `should reject: ${bad}`)
      assert.throws(() => JSON.parse(bad), `JSON.parse accepts: ${bad}`)
    }
  })
})

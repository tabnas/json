/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')

const Json = require('../dist/json.js')
const { parse, JsonError } = Json

describe('json', () => {
  it('default export is parse', () => {
    assert.strictEqual(Json.default, parse)
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
    assert.deepStrictEqual(parse('{"a":[1,{"b":2}]}'), { a: [1, { b: 2 }] })
  })

  it('handles surrogate pairs', () => {
    assert.strictEqual(parse('"\\uD83D\\uDE00"'), '😀')
  })

  it('handles all simple escapes', () => {
    assert.strictEqual(parse('"\\b\\f\\n\\r\\t\\/\\\\\\""'), '\b\f\n\r\t/\\"')
  })

  it('rejects empty input', () => {
    assert.throws(() => parse(''), (e) => e instanceof JsonError && e.code === 'unexpected_eof')
  })

  it('rejects whitespace-only input', () => {
    assert.throws(() => parse('   '), (e) => e.code === 'unexpected_eof')
  })

  it('reports line and column', () => {
    try {
      parse('{\n  "a": x\n}')
      assert.fail('should have thrown')
    } catch (e) {
      assert.ok(e instanceof JsonError)
      assert.strictEqual(e.line, 2)
      assert.strictEqual(e.code, 'unexpected_char')
    }
  })

  it('rejects extended grammar that jsonic would accept', () => {
    // unquoted key
    assert.throws(() => parse('{a:1}'))
    // trailing comma
    assert.throws(() => parse('[1,2,]'))
    // comment
    assert.throws(() => parse('1 // note'))
    // single quotes
    assert.throws(() => parse("'x'"))
    // implicit object
    assert.throws(() => parse('a:1,b:2'))
    // implicit array
    assert.throws(() => parse('x,y,z'))
    // hex number
    assert.throws(() => parse('0x10'))
  })

  it('non-string input throws', () => {
    assert.throws(() => parse(42), (e) => e instanceof JsonError && e.code === 'invalid_input')
  })
})

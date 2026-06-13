/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')
const { PassThrough } = require('node:stream')

const { run, main } = require('../dist/json-cli.js')

describe('cli', () => {
  it('run prints pretty JSON and returns 0', () => {
    let out = ''
    let err = ''
    const code = run('{"a":1,"b":[2,3]}', (s) => (out += s), (s) => (err += s))
    assert.strictEqual(code, 0)
    assert.strictEqual(out, JSON.stringify({ a: 1, b: [2, 3] }, null, 2) + '\n')
    assert.strictEqual(err, '')
  })

  it('run reports parse errors on stderr and returns 1', () => {
    let out = ''
    let err = ''
    const code = run('{a:1}', (s) => (out += s), (s) => (err += s))
    assert.strictEqual(code, 1)
    assert.strictEqual(out, '')
    assert.match(err, /unexpected/)
  })

  it('main parses command-line arguments', () => {
    let out = ''
    let exitCode = -1
    main(
      ['{"a":1}'],
      new PassThrough(),
      (s) => (out += s),
      () => {},
      (c) => (exitCode = c),
    )
    assert.strictEqual(exitCode, 0)
    assert.match(out, /"a": 1/)
  })

  it('main reads JSON from stdin', async () => {
    let out = ''
    const stdin = new PassThrough()
    const code = await new Promise((resolve) => {
      main([], stdin, (s) => (out += s), () => {}, resolve)
      stdin.write('[1,2,')
      stdin.end('3]')
    })
    assert.strictEqual(code, 0)
    assert.deepStrictEqual(JSON.parse(out), [1, 2, 3])
  })
})

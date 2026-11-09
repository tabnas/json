/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

// reader.test.js — a parse from a reader (`tn.parseReader`, admin ADR-30).
//
// `make()` declares `lex.window`, since every token of this grammar is the
// engine's own matchers' and its one check reads no further than a
// number's text, so a reader is read a window at a time. Every row of the
// shared fixtures and every document of the conformance corpus parses the
// same from a reader, handed over a few bytes at a time and under windows
// whose margin and block are a few units (`parseReaderWithSizes`, which
// makes every matcher meet the window's edge), as from its text: the same
// value, or an error with the same code, position, row and column. So do
// two long documents. rs/tests/reader_test.rs and go/reader_test.go hold
// the other runtimes to the same.

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const { findSpecDir, loadSpecDir } = require('@tabnas/support')

const { Tabnas, json, make } = require('../dist/json.js')

const SUITE = Path.join(
  __dirname, '..', '..', 'test', 'jsontestsuite', 'test_parsing')

// The pieces a reader hands its bytes over in.
const PIECES = [1, 3, 64]

// Windows whose margin and block are a few units.
const WINDOWS = [[1, 1], [3, 2], [64, 64]]

// A reader that hands its bytes over at most `size` at a time.
function pieces(bytes, size) {
  let at = 0
  return {
    read(buf) {
      const n = Math.min(size, buf.length, bytes.length - at)
      buf.set(bytes.subarray(at, at + n))
      at += n
      return n
    },
  }
}

// A value as text, with the sign of a negative zero kept and the keys in
// their order, so that two values compare as text.
function render(value) {
  if ('number' === typeof value && Object.is(value, -0)) return '-0'
  if (Array.isArray(value)) return '[' + value.map(render).join(',') + ']'
  if (null !== value && 'object' === typeof value) {
    return '{' + Object.keys(value)
      .map((k) => JSON.stringify(k) + ':' + render(value[k])).join(',') + '}'
  }
  return JSON.stringify(value)
}

// What a parse gave: its value, or its error's code, position, row and
// column. An error's excerpt is left out: a long line's may be cut where
// the window begins, the one thing a parse from a reader shows differently.
function outcome(parse) {
  try {
    return 'ok ' + render(parse())
  } catch (e) {
    if (null == e || 'function' !== typeof e.toJSON) throw e
    const d = e.toJSON()
    return `err ${d.code} at ${d.pos} ${d.row}:${d.col}`
  }
}

// Every way this test reads `bytes` from a reader, each with its outcome:
// in every size of pieces under the window's own sizes, and under every
// small window.
function readings(tn, bytes) {
  const out = []
  for (const size of PIECES) {
    out.push([`pieces of ${size}`,
      outcome(() => tn.parseReader(pieces(bytes, size)))])
    for (const [margin, block] of WINDOWS) {
      out.push([`pieces of ${size}, margin ${margin}, block ${block}`,
        outcome(() => tn.parseReaderWithSizes(
          pieces(bytes, size), { margin, block, chunk: size }))])
    }
  }
  return out
}

// Whether `bytes` are UTF-8, as the engine decodes a reader's bytes.
const utf8 = new TextDecoder('utf-8', { fatal: true })
function isUtf8(bytes) {
  try {
    utf8.decode(bytes)
    return true
  } catch {
    return false
  }
}

// A long document of `count` records, one a line, as rs/tests/reader_test.rs
// writes it.
function records(count) {
  const out = []
  for (let i = 0; i < count; i++) {
    out.push(`{"id":${i},"name":"ñ${i}","tags":["a","b"],"n":-1.5e3}`)
  }
  return '[' + out.join(',\n') + ']'
}

// A document of tokens each longer than a window's margin of 64 KiB, so
// that a window grows to hold one: a string of escapes and characters past
// ASCII, a number, a run of spaces and tabs, and a run of lines.
// go/reader_test.go, which cannot make a window small, places a token of
// every kind across the window's boundaries instead.
function longTokens() {
  return '["' + 'ab\\u00e9ñ€𝄞\\n'.repeat(5000) + '",' +
    '1.' + '1234567890'.repeat(7000) + ',' + ' \t'.repeat(40000) +
    '\n\r\n'.repeat(30000) + 'true]'
}

describe('reader', () => {
  it('make reads a reader a window at a time', () => {
    assert.equal(make().config().lex.window, true)
    // The plugin leaves the window to the parser that installs it, since
    // other grammars install it beneath matchers of their own.
    assert.equal(new Tabnas({ plugins: [json] }).config().lex.window, false)
  })

  it('every row parses the same from a reader', () => {
    const tn = make()
    let compared = 0
    for (const file of loadSpecDir(findSpecDir(__dirname))) {
      for (const row of file.rows) {
        const input = row.unescNamed('input')
        const whole = outcome(() => tn.parse(input))
        for (const [how, read] of readings(tn, Buffer.from(input, 'utf8'))) {
          assert.equal(read, whole, `${row.where()} in ${how}`)
          compared++
        }
      }
    }
    assert.ok(1000 < compared, '' + compared)
    console.log(`reader: ${compared} parses of the shared rows compared`)
  })

  it('every corpus document parses the same from a reader', () => {
    // A document that is not UTF-8 is no text a reader can give: a reader
    // is refused where its bytes stop being UTF-8, `unreadable`, or at the
    // document's own error where the parse meets that first. Every other
    // document reads as its text does. The two longer than 4 KiB, 100,000
    // and 250,000 open brackets that cost a second a parse here, are read
    // in pieces of 4 KiB.
    assert.ok(Fs.existsSync(SUITE),
      `nst/JSONTestSuite corpus not found at ${SUITE}: ` +
      'run sh test/fetch-jsontestsuite.sh')
    const tn = make()
    const files = Fs.readdirSync(SUITE).filter((f) => f.endsWith('.json')).sort()
    let compared = 0
    let refused = 0
    let long = 0
    files.forEach((name, index) => {
      const bytes = Fs.readFileSync(Path.join(SUITE, name))
      const whole = outcome(() => tn.parse(bytes.toString('utf8')))
      let reads
      if (4096 < bytes.length) {
        reads = [['pieces of 4096',
          outcome(() => tn.parseReader(pieces(bytes, 4096)))]]
        long++
      } else {
        reads = readings(tn, bytes)
      }
      if (isUtf8(bytes)) {
        for (const [how, read] of reads) {
          assert.equal(read, whole, `${name} in ${how}`)
          compared++
        }
      } else {
        const unreadable = new Set()
        for (const [how, read] of reads) {
          if (read.startsWith('err unreadable ')) {
            unreadable.add(read)
          } else {
            assert.equal(read, whole, `${name} in ${how}`)
          }
        }
        assert.ok(unreadable.size <= 1, `${name}: ${[...unreadable]}`)
        refused++
      }
      if (0 === (index + 1) % 50 || index + 1 === files.length) {
        console.log(`reader corpus: ${index + 1} of ${files.length} ` +
          `(${Math.floor(((index + 1) * 100) / files.length)}%)`)
      }
    })
    assert.equal(files.length, 318)
    assert.equal(refused, 25)
    assert.equal(long, 2)
    assert.ok(3000 < compared, '' + compared)
  })

  it('a long document parses the same from a reader', () => {
    // 5,000 records, some 300 KB, several windows of 64 KiB.
    const tn = make()
    const text = records(5000)
    const bytes = Buffer.from(text, 'utf8')
    const whole = outcome(() => tn.parse(text))
    assert.ok(whole.startsWith('ok '), whole.slice(0, 80))
    assert.equal(outcome(() => tn.parseReader(pieces(bytes, 4096))), whole)
    assert.equal(outcome(() => tn.parseReaderWithSizes(
      pieces(bytes, 5), { margin: 16, block: 16, chunk: 7 })), whole)
  })

  it('tokens longer than the window parse the same from a reader', () => {
    // As written, with an error after them, and cut inside the string.
    const tn = make()
    const text = longTokens()
    const docs = [
      ['as written', text, 'ok '],
      ['with an error after them', text.slice(0, -1) + ',]', 'err '],
      ['cut inside the string', text.slice(0, 50000), 'err '],
    ]
    for (const [what, doc, kind] of docs) {
      const bytes = Buffer.from(doc, 'utf8')
      const whole = outcome(() => tn.parse(doc))
      assert.ok(whole.startsWith(kind), `${what}: ${whole.slice(0, 80)}`)
      for (const size of [4096, 64 * 1024 + 7]) {
        assert.equal(outcome(() => tn.parseReader(pieces(bytes, size))),
          whole, `${what}, in pieces of ${size}`)
      }
      assert.equal(outcome(() => tn.parseReaderWithSizes(
        pieces(bytes, 5), { margin: 16, block: 16, chunk: 7 })),
      whole, `${what}, in a small window`)
    }
  })
})

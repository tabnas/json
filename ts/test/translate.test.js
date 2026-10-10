/* Copyright (c) 2026 tabnas, MIT License */

const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const { test } = require('node:test')

const { translate } = require('../dist/json')

const root = path.resolve(__dirname, '..', '..')

test('translation parts expose the manifest and builtin render entry', () => {
  const parts = translate()
  assert.ok(parts)
  assert.equal(parts.manifest, readFileSync(path.join(root, 'tabnas.plugin.json'), 'utf8'))
  assert.equal(parts.lift, undefined)
  assert.equal(parts.render?.entry, 'json')
  assert.equal(parts.render?.source, undefined)
})

// An embed takes a plain tree into a format's own schema. JSON's events
// carry a plain tree, so its manifest names none and the package carries
// none; a manifest that named one would be held to its file here.
test('translation parts carry the embed the manifest names, and none where it names none', () => {
  const parts = translate()
  const spec = JSON.parse(readFileSync(path.join(root, 'tabnas.plugin.json'), 'utf8')).translate
  if (null == spec.embed) {
    assert.equal(parts.embed, undefined)
  } else {
    assert.equal(parts.embed?.entry, 'json-embed')
    assert.equal(parts.embed?.source, readFileSync(path.join(root, spec.embed), 'utf8'))
  }
})

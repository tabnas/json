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

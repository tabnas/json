/* Copyright (c) 2026 tabnas, MIT License */
'use strict'

const Fs = require('node:fs')
const Path = require('node:path')

// Load a tab-separated conformance fixture from test/spec. The first row
// is a header (input<TAB>expected); each subsequent row is a case. The
// expected column is either a JSON literal or `ERROR:<code>`.
function loadTSV(name) {
  const file = Path.join(__dirname, 'spec', name + '.tsv')
  if (!Fs.existsSync(file)) {
    throw new Error('spec file not found: ' + file)
  }
  const text = Fs.readFileSync(file, 'utf8')
  const lines = text.split('\n').filter((line) => line.length > 0)
  const entries = []
  for (let i = 1; i < lines.length; i++) {
    const cols = lines[i].split('\t')
    entries.push({ row: i, cols })
  }
  return entries
}

module.exports = { loadTSV }

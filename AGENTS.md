# Agents Guide — json

## What this project is

`json` is a **standard JSON parser**: it accepts exactly the grammar of
[RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) / ECMA-404 and rejects
everything else. There is deliberately **no extended grammar** — no
comments, trailing commas, unquoted keys, single-quoted or multiline
strings, implicit objects/arrays, or non-decimal numbers. The bar is
parity with the platform JSON parsers (`JSON.parse` in TS/JS,
`encoding/json` in Go).

It is a **grammar plugin** for the
[`tabnas`](https://github.com/tabnas/parser) parsing engine. The engine
ships no grammar; this package supplies the standard-JSON one in both
runtimes. The rule set (`val` / `map` / `list` / `pair` / `elem`) is
jsonic's **"Plain JSON"** grammar (the pure-JSON core in jsonic's
`grammar.ts`, before it is extended for the relaxed jsonic format),
installed on its own with the lexer restricted to strict JSON.

This repository was created from the
[`tabnas/jsonic`](https://github.com/tabnas/jsonic) template and
refactored down to standard JSON, dropping jsonic's extended grammar.

## Repository map

| Path | What it is |
|---|---|
| [`ts/`](ts/) | **Canonical** TypeScript implementation — the `@tabnas/json` package. Plugin in `src/json.ts`, CLI in `src/json-cli.ts`. Depends on the `tabnas` npm package. |
| [`go/`](go/) | Go port — `github.com/tabnas/json/go`. Plugin in `json.go`. Depends on `github.com/tabnas/parser/go` via a `replace` directive (sibling checkout). |
| [`ts/test/spec/`](ts/test/spec/) | Shared `.tsv` conformance fixtures (`input → expected`, or `ERROR`). Run by both suites. |

## The tabnas engine dependency

Both runtimes depend on the engine as a **sibling checkout**, the same
development model jsonic uses, until `tabnas/parser` publishes tagged
packages:

- TypeScript: `"tabnas": "file:../../parser/ts"` in `ts/package.json`.
- Go: `replace github.com/tabnas/parser/go => ../../parser/go` in
  `go/go.mod`.

Clone `https://github.com/tabnas/parser` as a sibling of this repo, build
the engine's TS (`cd parser/ts && npm install && npm run build`), then
work here. CI (`.github/workflows/build.yml`) checks the engine out as a
sibling and builds it first.

## Authority and alignment rules

1. **TypeScript is canonical.** When TS and Go disagree on parse
   behavior, TS wins; change Go to match, and add or extend a shared
   fixture when the behavior is expressible as `input → output`.
2. The shared fixtures in `ts/test/spec/*.tsv` are the parity contract.
   Both suites run them and both must stay green. The Go suite resolves
   them at `../ts/test/spec` (see `go/json_test.go` `specDir`).
3. Error **codes** are engine-specific and are a known difference between
   the runtimes, so the shared error fixtures assert only that an input
   is *rejected* (`ERROR`), not a particular code.
4. Stay standard. Any change that would accept input `JSON.parse` /
   `encoding/json` reject (or reject input they accept) is a bug. The
   strict-number tightening lives in `JSON_OPTIONS` / `jsonOptions`
   (`number.exclude`): keep TS regex and Go predicate in sync.
5. Keep the grammar a reusable foundation. `registerJsonGrammar` (TS) /
   `RegisterJSONGrammar` (Go) install only the JSON core so other plugins
   can layer on it; don't fold options-specific behavior into the rules.

## Build & test

TypeScript:

```bash
cd ts && npm install && npm test   # tsc build + node --test
```

Go:

```bash
cd go && go test ./...             # also runs the shared spec fixtures
```

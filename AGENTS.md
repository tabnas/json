# Agents Guide — json

## What this project is

`json` is a **standard JSON parser**: it accepts exactly the grammar of
[RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) / ECMA-404 and rejects
everything else. There is deliberately **no extended grammar** — no
comments, trailing commas, unquoted keys, single-quoted or multiline
strings, implicit objects/arrays, or non-decimal numbers. Keep that
constraint in mind for every change: the shared fixtures encode exactly
the standard-JSON behavior, and the bar is parity with the platform JSON
parsers (`JSON.parse` in TS/JS, `encoding/json` in Go).

This repository was created from the
[`tabnas/jsonic`](https://github.com/tabnas/jsonic) lenient-parser
template and refactored down to standard JSON, dropping jsonic's
configurable grammar/plugin engine. Each runtime is now a small,
self-contained recursive-descent parser with no external engine
dependency.

## Repository map

| Path | What it is |
|---|---|
| [`ts/`](ts/) | **Canonical** TypeScript implementation — the `@tabnas/json` package. Parser in `src/json.ts`, CLI in `src/json-cli.ts`. |
| [`go/`](go/) | Go port — `github.com/tabnas/json/go`. Parser in `json.go`, error type in `error.go`. |
| [`ts/test/spec/`](ts/test/spec/) | Shared `.tsv` conformance fixtures (`input → expected`, or `ERROR:<code>`). Run by both suites. |

## Authority and alignment rules

1. **TypeScript is canonical.** When TS and Go disagree on parse
   behavior, TS wins; change Go to match, and add or extend a shared
   fixture when the behavior is expressible as `input → output`.
2. The shared fixtures in `ts/test/spec/*.tsv` are the parity contract.
   Both suites run them and both must stay green. The Go suite resolves
   them at `../ts/test/spec` (see `go/json_test.go` `specDir`).
3. Error `code` values are part of the contract — the error fixtures
   assert them. If you rename or add a code, update both parsers and the
   fixtures together.
4. Stay standard. Any change that would accept input `JSON.parse` /
   `encoding/json` reject (or reject input they accept) is a bug unless
   the platform parsers are themselves wrong on that input.

## Build & test

TypeScript:

```bash
cd ts && npm install && npm test   # tsc build + node --test
```

Go:

```bash
cd go && go test ./...             # also runs the shared spec fixtures
```

Both run in CI (`.github/workflows/build.yml`) across Linux/macOS/Windows.

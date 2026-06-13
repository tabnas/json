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
3. Error **codes** are part of the shared contract. `json-errors.tsv` is
   `input → code`, and both suites assert the exact code
   (`unexpected`, `unterminated_string`, `invalid_ascii`,
   `invalid_unicode`). The runtimes are required to reject the same input
   with the same code; if you add an error fixture, verify the code is
   identical in both runtimes before committing it.
4. Stay standard. Any change that would accept input `JSON.parse` /
   `encoding/json` reject (or reject input they accept) is a bug. Two
   pieces of `JSON_OPTIONS` / `jsonOptions` enforce strictness the engine
   defaults leave open — keep them in sync across runtimes:
   - `number.exclude` (TS regex / Go predicate) rejects non-standard
     numbers (`+1`, `.5`, `1.`, `01`, `00`).
   - `string.escapeStrict: true` plus dropping `v` / `'` / `` ` `` from
     the escape map (`escape: { v: '', "'": '', '`': '' }`) and
     `allowUnknown: false` restrict escapes to exactly the standard JSON
     set. `escapeStrict` disables the engine's `\xHH` and `\u{...}`
     structural escapes (plain `\uXXXX`, including surrogate pairs,
     stays).
5. Keep the grammar a reusable foundation. `registerJsonGrammar` (TS) /
   `RegisterJSONGrammar` (Go) install only the JSON core so other plugins
   can layer on it; don't fold options-specific behavior into the rules.

## String escapes

Both runtimes accept exactly the standard JSON escapes (`\" \\ \/ \b \f
\n \r \t \uXXXX`) and reject everything else — unknown escapes (`\q`,
`\z` → `unexpected`), the non-standard built-ins (`\v`, `\'`, `` \` `` →
`unexpected`), the `\xHH` ASCII escape (`\x41` → `unexpected`), and the
`\u{...}` braced form (`\u{41}` → `invalid_unicode`, since `{` is not a
hex digit on the plain `\uXXXX` path). This requires the `tabnas` engine's
`string.escapeStrict` option (added on `main`); the strict config is set
in `JSON_OPTIONS` / `jsonOptions`. These rejections are covered by
`json-errors.tsv` with shared codes. Do not add these escapes to
`json-valid.tsv` — the valid runner cross-checks against the platform
parser, which rejects them.

## Build & test

TypeScript:

```bash
cd ts && npm install && npm test   # tsc build + node --test
```

Go:

```bash
cd go && go test ./...             # also runs the shared spec fixtures
```

## Coverage

Both runtimes keep the plugin layer at ≥95% line coverage (the parsing
engine is a dependency with its own suite, so it is out of scope here):

```bash
cd ts && npm run coverage          # node --test, enforces lines ≥ 95%
cd go && go test -cover ./...      # currently 100% of statements
```

The grammar action closures (`registerJsonGrammar` /
`RegisterJSONGrammar`) are kept standard-only: branches that handle
inputs the strict lexer cannot produce (empty values, non-string keys)
were removed, so the rule actions stay reachable and covered. Keep it
that way — don't reintroduce dead extended-grammar handling.

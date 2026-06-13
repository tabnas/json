# @tabnas/json

A standard JSON parser (RFC 8259 / ECMA-404) for TypeScript and
JavaScript — the standard-JSON **grammar plugin** for the
[`tabnas`](https://github.com/tabnas/parser) parsing engine.

Available for [TypeScript/JavaScript](#install) and [Go](../go/).

## Install

```bash
npm install @tabnas/json
```

`tabnas` (the engine) is a peer/dependency; see [Develop](#develop) for
the sibling-checkout setup used until it is published.

## Quick example

```ts
import { parse } from '@tabnas/json'

parse('{"a":1, "b":2}')              // { a: 1, b: 2 }
parse('[1, 2, 3]')                   // [1, 2, 3]
parse('{"a": {"b": [true, null]}}')  // { a: { b: [true, null] } }
```

```js
const { parse } = require('@tabnas/json')
parse('"hello"') // "hello"
```

`parse` is also the default export.

## Use it as a plugin

The package is a `tabnas` grammar plugin. Install it on your own engine
instance and layer further grammar on the shared rules:

```ts
import { Tabnas } from 'tabnas'
import { json } from '@tabnas/json'

const am = new Tabnas({ plugins: [json] })
am.parse('{"a":[1,2,3]}')
```

`registerJsonGrammar(am)` installs just the `val` / `map` / `list` /
`pair` / `elem` rules (jsonic's "Plain JSON" core) without the strict
options, for plugins that want to build on the JSON rule set.

## What it accepts

Exactly standard JSON:

- objects `{ "key": value, ... }` with double-quoted string keys
- arrays `[ value, ... ]`
- double-quoted strings with the JSON escapes
  (`\" \\ \/ \b \f \n \r \t \uXXXX`, including surrogate pairs)
- numbers: optional `-`, integer (no leading zeros), optional fraction,
  optional `e`/`E` exponent
- `true`, `false`, `null`
- insignificant whitespace: space, tab, line feed, carriage return

It **rejects** everything outside that grammar — comments, trailing
commas, unquoted keys, single-quoted or backtick strings, multiline
strings, implicit objects/arrays, hex/octal/binary numbers, leading
zeros, a leading `+`, a bare `.5` or trailing `1.`, and empty input.
This matches the platform `JSON.parse`.

## Errors

On invalid input, `parse` throws a `TabnasError` (also exported as
`JsonError`) carrying `code`, `lineNumber`, and `columnNumber`:

```ts
import { parse, TabnasError } from '@tabnas/json'

try {
  parse('{a:1}')
} catch (err) {
  if (err instanceof TabnasError) {
    err.code // 'unexpected'
  }
}
```

## CLI

```bash
echo '{"a":1}' | npx tabnas-json
tabnas-json '{"a":1}'
```

Prints the re-serialized (pretty) JSON, or the error on stderr with exit
code 1.

## Develop

This package depends on the engine as a sibling checkout:

```bash
git clone https://github.com/tabnas/parser   # sibling of this repo
( cd parser/ts && npm install && npm run build )
npm install      # resolves "tabnas": "file:../../parser/ts"
npm test         # tsc build + node --test
```

See [`AGENTS.md`](AGENTS.md) for layout and conventions.

## License

MIT.

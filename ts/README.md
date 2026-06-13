# @tabnas/json

A standard JSON parser (RFC 8259 / ECMA-404) for TypeScript and
JavaScript. Strict by design — no extended grammar, no dependencies.

Available for [TypeScript/JavaScript](#install) and [Go](../go/).

## Install

```bash
npm install @tabnas/json
```

## Quick example

```ts
import { parse } from '@tabnas/json'

parse('{"a":1, "b":2}')          // { a: 1, b: 2 }
parse('[1, 2, 3]')               // [1, 2, 3]
parse('{"a": {"b": [true, null]}}') // { a: { b: [true, null] } }
```

```js
const { parse } = require('@tabnas/json')
parse('"hello"') // "hello"
```

`parse` is also the default export:

```ts
import parse from '@tabnas/json'
```

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
zeros, a leading `+`, and empty input. This is the same surface as the
platform `JSON.parse`.

## Errors

On invalid input, `parse` throws a `JsonError` with a machine-readable
`code` and the source position:

```ts
import { parse, JsonError } from '@tabnas/json'

try {
  parse('{a:1}')
} catch (err) {
  if (err instanceof JsonError) {
    err.code   // 'expected_key'
    err.line   // 1
    err.column // 2
    err.index  // 1
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

```bash
npm install
npm run build   # tsc → dist/
npm test        # build + node --test
```

See [`AGENTS.md`](AGENTS.md) for layout and conventions.

## License

MIT.

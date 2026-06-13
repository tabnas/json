# json

A standard JSON parser — strict, fast, and dependency-free — for
TypeScript/JavaScript and Go.

```
{"a":1,"foo":"bar"}  →  { a: 1, foo: 'bar' }
```

This parser implements exactly the JSON grammar defined by
[RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) / ECMA-404 and nothing
more. It accepts objects, arrays, strings, numbers, and the literals
`true`, `false` and `null`. It rejects everything an extended grammar
would relax: comments, trailing commas, unquoted keys, single-quoted and
multiline strings, implicit objects and arrays, hex/octal numbers. If
`JSON.parse` (TS/JS) or `encoding/json` (Go) would reject the input, so
does this parser.

> This repository began life from the
> [`tabnas/jsonic`](https://github.com/tabnas/jsonic) template — a
> lenient, extensible JSON parser — and was refactored down to a standard
> JSON parser by removing the extended grammar in both runtimes.

## Choose your runtime

| Runtime | Start here |
|---|---|
| **TypeScript / JavaScript** | [`ts/README.md`](ts/README.md) |
| **Go** (`github.com/tabnas/json/go`) | [`go/README.md`](go/README.md) |

Both runtimes are self-contained — no parsing-engine dependency — and
produce identical results. TypeScript is canonical: both suites run the
shared conformance fixtures in [`ts/test/spec/`](ts/test/spec/).

## Quick example

TypeScript / JavaScript:

```ts
import { parse } from '@tabnas/json'

parse('{"a":1,"b":[2,3]}') // { a: 1, b: [2, 3] }
```

Go:

```go
import json "github.com/tabnas/json/go"

v, err := json.Parse(`{"a":1,"b":[2,3]}`)
```

## Contributing

Each directory has an `AGENTS.md` with build, layout, and contribution
notes; start with [`AGENTS.md`](AGENTS.md).

## License

MIT.

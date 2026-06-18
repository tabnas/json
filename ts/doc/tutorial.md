# Tutorial: parsing your first JSON with `@tabnas/json`

This is a learning-oriented walkthrough. You start with nothing and end
with a working program that parses JSON, handles an error, and extends
the parser. Follow it top to bottom; every step builds on the previous
one.

If you only want a reference for the API, read [`reference.md`](reference.md).
If you have a specific problem to solve, read [`guide.md`](guide.md).
To understand *how and why* it works, read [`concepts.md`](concepts.md).

## What you are building

`@tabnas/json` is a standard JSON parser — it accepts exactly what
`JSON.parse` accepts (RFC 8259 / ECMA-404) and nothing more. By the end
of this tutorial you will have used it to turn JSON text into JavaScript
values, caught a parse error, and built a JSON-with-comments parser on
top of it.

## Step 1 — Install

```bash
npm install @tabnas/json
```

The `tabnas` parsing engine (`@tabnas/parser`) is a peer dependency and is
installed automatically with npm 7+ on Node 24+. (During local
development against unpublished engine builds, see the "Develop" section
of [`../README.md`](../README.md).)

## Step 2 — Parse a value

The whole library is reachable through one function: `parse`. Give it a
JSON string, get back the JavaScript value.

```js
const { parse } = require('@tabnas/json')

parse('42')      // => 42
parse('"hello"') // => "hello"
parse('true')    // => true
parse('null')    // => null
```

`parse` is also the default export, so in an ES module you can write
`import parse from '@tabnas/json'`.

## Step 3 — Parse objects and arrays

The same `parse` handles structured data. Objects become plain objects,
arrays become arrays, and nesting works to any depth.

```js
const { parse } = require('@tabnas/json')

parse('{"a":1,"b":2}')              // => { a: 1, b: 2 }
parse('[1, 2, 3]')                  // => [1, 2, 3]
parse('{"a": {"b": [true, null]}}') // => { a: { b: [true, null] } }
```

Insignificant whitespace (spaces, tabs, newlines) between tokens is
ignored, exactly as in standard JSON.

## Step 4 — See what gets rejected

This parser is *strict*. Anything `JSON.parse` would reject, it rejects
too. Try a few inputs that look like JSON but are not:

```js
const { parse } = require('@tabnas/json')

// Each of these THROWS — they are not standard JSON:
//   parse('{a:1}')    unquoted key
//   parse('[1,2,]')   trailing comma
//   parse("'x'")      single-quoted string
//   parse('01')       leading zero
//   parse('1 // hi')  comment
```

That strictness is the point: `@tabnas/json` is the baseline that every
relaxed variant (like `@tabnas/jsonic`) extends.

## Step 5 — Handle a parse error

Invalid input throws a `TabnasError`. Catch it and inspect the structured
fields — `code`, `lineNumber`, `columnNumber`:

```js
const { parse, TabnasError } = require('@tabnas/json')

let code
try {
  parse('{a:1}') // unquoted key — invalid
} catch (err) {
  if (err instanceof TabnasError) {
    code = err.code // 'unexpected'
  }
}
code // => "unexpected"
```

`TabnasError` is also exported under the alias `JsonError` if you prefer
that name. The `err.message` is a human-readable, source-pointing message.

## Step 6 — Build your own parser instance

`parse` uses one shared, lazily-built engine. When you want to customize
the parser, build your own instance with `make`. Extra options are
applied on top of the strict JSON configuration:

```js
const { make } = require('@tabnas/json')

const p = make()
p.parse('[1,2]') // => [1, 2]
```

An instance is reusable — build it once, call `parse` on it many times.

## Step 7 — Extend the grammar (JSON-with-comments)

Here is the foundation idea in action. The `json` plugin installs the
JSON grammar onto a bare `tabnas` engine; you can then layer on extra
behavior. Re-enabling comment lexing turns strict JSON into JSONC:

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')

const jsonc = new Tabnas({ plugins: [json] })
jsonc.options({ comment: { lex: true } })

jsonc.parse('{"a":1} // a note')   // => { a: 1 }
jsonc.parse('{"a":/* inline */2}') // => { a: 2 }
```

The base `parse` still rejects those comments — you extended a *copy*,
not the global parser.

## Step 8 — Use the command line

The package ships a tiny CLI, `tabnas-json`. It reads JSON from an
argument or from stdin and prints the re-serialized, pretty-printed form:

```bash
tabnas-json '{"a":1,"b":[2,3]}'
# {
#   "a": 1,
#   "b": [
#     2,
#     3
#   ]
# }

echo '[1,2,3]' | npx tabnas-json
```

Invalid input prints the error to stderr and exits with code 1.

## Where to go next

- [`guide.md`](guide.md) — task-focused recipes (error handling,
  metadata, building other parsers).
- [`reference.md`](reference.md) — the exact API and CLI surface.
- [`concepts.md`](concepts.md) — how the grammar-plugin model works and
  why this is the strict baseline.

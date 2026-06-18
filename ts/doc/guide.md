# How-to guide: `@tabnas/json` recipes

Task-oriented recipes for real problems. Each section is self-contained.
For a guided introduction read [`tutorial.md`](tutorial.md); for the full
API see [`reference.md`](reference.md); for the design see
[`concepts.md`](concepts.md).

## Parse a string and use the result

`parse` returns ordinary JavaScript values: numbers, strings, booleans,
`null`, arrays, and objects.

```js
const { parse } = require('@tabnas/json')

const order = parse('{"id":7,"items":["pen","pad"],"paid":true}')
order.id       // => 7
order.items[1] // => "pad"
order.paid     // => true
```

## Handle invalid input

Invalid JSON throws a `TabnasError`. Wrap `parse` and read the structured
fields rather than scraping the message:

```js
const { parse, TabnasError } = require('@tabnas/json')

function tryParse(src) {
  try {
    return { ok: true, value: parse(src) }
  } catch (err) {
    if (err instanceof TabnasError) {
      return { ok: false, code: err.code }
    }
    throw err
  }
}

tryParse('[1,2,3]').ok    // => true
tryParse('[1,2,]').ok     // => false
tryParse('[1,2,]').code   // => "unexpected"
tryParse('"abc').code     // => "unterminated_string"
```

Each error carries `code`, `lineNumber`, and `columnNumber` to locate the
problem. The three codes this parser emits are `unexpected`,
`unterminated_string`, and `invalid_unicode`. The default export and the
`JsonError` alias both reference the same `TabnasError` class.

## Reuse a parser efficiently

The top-level `parse` already reuses a single lazily-built engine, so
repeated calls do not rebuild the grammar. When you need a *configured*
parser, build one with `make` and keep it around:

```js
const { make } = require('@tabnas/json')

const p = make()
p.parse('{"x":1}') // => { x: 1 }
p.parse('{"y":2}') // => { y: 2 }
```

Parsing creates a fresh context each call, so a single instance is safe to
reuse across many parses.

## Keep introspection metadata (info options)

By default the parser produces plain values. If you need to know whether a
container was explicit or capture a string's quote character, enable the
engine's `info` options through `make`. The values then carry an
`__info__` marker (a non-enumerable property, so it does not affect
`JSON.stringify`):

```js
const { make } = require('@tabnas/json')

const p = make({ info: { map: true, list: true, text: true } })
const out = p.parse('{"a":["x",1]}')

// The metadata marker on the object records implicit:false (the map
// was written explicitly with braces):
const mark = Object.getOwnPropertyDescriptor(out, '__info__')
mark.value.implicit // => false
```

String values become boxed `String` objects whose marker records the
quote (`"`); numbers and the keywords stay primitive. This is exactly the
mode that downstream plugins use when they need to preserve syntax detail.

## Build a JSON-with-comments (JSONC) parser

The `json` plugin is a foundation. Install it on a bare engine, then turn
comment lexing back on to accept `//` and `/* */` comments:

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')

const jsonc = new Tabnas({ plugins: [json] })
jsonc.options({ comment: { lex: true } })

jsonc.parse('{"a":1} // trailing note') // => { a: 1 }
jsonc.parse('{"a":/* inline */2}')      // => { a: 2 }
```

This changes only your instance. The package-level `parse` is unaffected
and still rejects comments.

## Install the grammar without the strict options

`json` does two things: applies strict JSON lexer options *and* registers
the rule set. When you want only the rules — to extend them under your own
lexer configuration — call `registerJsonGrammar` directly:

```js
const { Tabnas } = require('@tabnas/parser')
const { registerJsonGrammar } = require('@tabnas/json')

const tn = new Tabnas()
registerJsonGrammar(tn) // installs val / map / list / pair / elem only
tn.parse('{"a":[1,2,3]}') // => { a: [1, 2, 3] }
```

From here you can use the engine's rule API (`tn.rule(...)` and the
`@<rule>-<phase>` hooks) to extend the shared `val` / `map` / `list` /
`pair` / `elem` rules without redeclaring the JSON core. See
[`concepts.md`](concepts.md) for what each rule does.

## Pretty-print from the command line

The `tabnas-json` CLI parses and re-serializes with a 2-space indent.

```bash
# From an argument:
tabnas-json '{"a":1,"b":[2,3]}'

# From stdin (a pipeline):
curl -s https://example.com/data.json | tabnas-json
```

On invalid input it writes the error to stderr and exits with code 1, so
it composes cleanly in shell pipelines and `set -e` scripts.

## Guard against prototype pollution

Parsed objects use a `null` prototype (`Object.create(null)`). A
`"__proto__"` key therefore becomes a normal own property instead of
mutating the prototype chain — there is no prototype-pollution gadget:

```js
const { parse } = require('@tabnas/json')

const out = parse('{"__proto__":{"polluted":true}}')
Object.hasOwn(out, '__proto__') // => true
({}).polluted                   // => undefined
```

The trade-off: parsed objects have no `Object.prototype`, so
`obj.hasOwnProperty(...)` is `undefined`. Use `Object.hasOwn(obj, key)`
(shown above) or `Object.keys(obj)` instead.

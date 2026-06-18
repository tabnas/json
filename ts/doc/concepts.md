# Concepts: how `@tabnas/json` works and why

Understanding-oriented. This explains the design — the engine
relationship, the grammar-plugin model, the role of the `$`-builtin
actions, and the trade-offs behind "strict". For hands-on material see
[`tutorial.md`](tutorial.md) and [`guide.md`](guide.md); for the exact API
see [`reference.md`](reference.md).

## The big idea: a grammar plugin, not a parser

The `tabnas` engine (`@tabnas/parser`) ships **no grammar of its own**. It
is a configurable lexer plus a rule-driven parser, and it does nothing
useful until a *grammar plugin* tells it what tokens to recognize and what
rules to apply. `@tabnas/json` is that plugin for standard JSON.

So `@tabnas/json` is two things bundled into the `json` plugin:

1. **A lexer configuration** (the strict `JSON_OPTIONS`) that restricts the
   engine to JSON tokens: double-quoted strings, decimal numbers, the
   keywords, no comments, no bare text.
2. **A rule set** (`val` / `map` / `list` / `pair` / `elem`) registered
   via `registerJsonGrammar`, describing how those tokens nest into
   values.

`parse` and `make` are thin conveniences over "make an engine, install the
`json` plugin."

## Where the grammar comes from

The rule set is not invented here — it is jsonic's **"Plain JSON"**
grammar. [`@tabnas/jsonic`](https://github.com/tabnas/jsonic) defines a
pure-JSON core and then extends it for its relaxed format (comments,
unquoted keys, trailing commas, implicit structures, single/backtick
strings, path diving). This repository takes that pure core, installs it
on its own, and clamps the lexer down to strict JSON — and stops there.

That lineage is the key to understanding the package: **`json` is the
baseline that `jsonic` relaxes.** Same `val` / `map` / `list` / `pair` /
`elem` rules; jsonic adds alternates and re-opens lexer options, `json`
keeps them closed.

## The five rules

The grammar is five small rules. Each is a state machine with *open*
alternates (entering the rule) and *close* alternates (leaving it). The
start rule is `val`.

- **`val`** — a value is a `map` (sees `{`), a `list` (sees `[`), or a
  plain scalar token (`#VAL`: a string, number, or keyword). On close it
  resolves to its built child or its scalar.
- **`map`** — an object. Opens on `{`; either closes immediately (`{}`) or
  matches `pair` rules; closes on `}`.
- **`list`** — an array. Opens on `[`; either closes immediately (`[]`) or
  matches `elem` rules; closes on `]`.
- **`pair`** — one `"key": value` entry. Matches `#KEY #CL` (a key token
  then a colon), parses a `val`, and on a comma loops to the next pair.
- **`elem`** — one list element. Parses a `val`, and on a comma loops to
  the next element.

This is why the package is a *foundation*: any plugin that wants
JSON-shaped structure can install these rules and add its own alternates
(e.g. an alternate on `val` for a new literal form, or on `pair` for a new
key syntax).

## How the value tree is built: the `$`-builtin actions

A grammar rule recognizes structure but does not, by itself, construct
JavaScript values. In older designs each rule carried a hand-written
closure to allocate objects, assign keys, and box strings. This grammar
has **no grammar-local closures at all**. Instead each alternate names one
of the engine's native-value **`$`-builtins**, and the engine merges the
real builder in at load time:

| Builtin | What it does |
|---|---|
| `@reset$` | Clear the parent-seeded node, so a scalar value does not inherit the parent container. |
| `@object$` | Allocate an empty object into the node. |
| `@array$` | Allocate an empty array into the node. |
| `@key$` | Capture the matched key token into a scratch slot for the pending assignment. |
| `@setval$` | Assign the just-built child value into the object under the captured key. |
| `@push$` | Append the just-built child value to the array. |
| `@value$` | Resolve the rule's value: a built child wins, else the scalar token. |

These builders are **info-aware**. When the engine's `info.map` /
`info.list` / `info.text` options are off (the default for strict JSON),
they build plain objects, arrays, and primitive strings. When those
options are on, the *same* builders attach the introspection markers —
container `implicit` flags, string `quote` info — without any change to
the grammar. That is why `make({ info: { ... } })` works: you flip engine
options and the shared builders do the extra bookkeeping. The grammar
spec declares `v: 2`, the schema version of the builtins it binds to.

## What "strict" buys, and what it costs

The whole point is parity with the platform parser. The engine's lexer is
*lenient* by default — it will happily tokenize hex numbers, bare text,
single quotes, and more. Strictness comes from a handful of options that
close those doors:

- `number.exclude` is a negative-lookahead regex (Go: a predicate over the
  same pattern) that rejects any number token not of the exact form
  `-?(0|[1-9][0-9]*)(.[0-9]+)?([eE][+-]?[0-9]+)?`. That single line
  rejects `+1`, `.5`, `1.`, `01`, and `00`.
- `string.escapeStrict: true`, plus dropping `v` / `'` / `` ` `` from the
  escape map and `allowUnknown: false`, narrows escapes to exactly the
  JSON set. `escapeStrict` disables the engine's structural `\xHH` and
  `\u{...}` escapes (plain `\uXXXX`, surrogate pairs included, stays).
- `text.lex`, `comment.lex`, `map.extend`, and `lex.empty` are turned off;
  `tokenSet.KEY` forces keys to be quoted strings only.

The trade-off is deliberate: this parser will **never** accept input that
`JSON.parse` rejects, and never reject input it accepts. If you want
leniency, you do not loosen `json` — you reach for `jsonic`, or you layer
your own options on top (the JSONC example in [`guide.md`](guide.md)).

## Why `null`-prototype objects

Parsed objects are created with `Object.create(null)`. This makes the
parser prototype-pollution-safe: a malicious `"__proto__"` key in the
input lands as an ordinary own property and cannot mutate any prototype
chain. The visible cost is that parsed objects lack `Object.prototype`, so
`obj.hasOwnProperty` is `undefined`; use `Object.hasOwn(obj, key)`. This
is a security-positive default, chosen over `JSON.parse`'s plain-object
behavior on purpose.

## Why a shared default instance

`parse` keeps one lazily-built engine in a module-level variable and
reuses it. Building an engine compiles the grammar, which is not free; a
parse, by contrast, creates a fresh per-call context and only reads
instance state. So the instance is safe to share and reusing it avoids
paying the build cost on every call. `make` exists for when you need a
*configured* instance instead of the shared default.

## TypeScript is canonical; Go tracks it

The TS implementation in `ts/src/json.ts` is the source of truth; the Go
port in `go/json.go` mirrors it line-for-line where it can. Both suites
run the same conformance fixtures in `ts/test/spec/*.tsv`
(`json-valid.tsv` = input → expected output, `json-errors.tsv` = input →
error code). The error **codes** are part of that shared contract: both
runtimes must reject the same input with the same code. See the Go
concepts doc's "Differences from the TS version" section for the small,
unavoidable runtime differences.

## The CLI

`tabnas-json` is a thin front end over `parse`. The logic is split into a
pure `run` (source string in, exit code out, output via injected sinks)
and a `main` that wires `run` to `process` (argv or stdin). Splitting it
this way makes both halves testable in-process without spawning a
subprocess — `run` is just a function. It parses, then re-serializes with
`JSON.stringify(value, null, 2)`, so its output is itself canonical JSON.

# Concepts: how `tabnasjson` works and why (Go)

Understanding-oriented. This explains the design — the engine
relationship, the grammar-plugin model, the role of the `$`-builtin
actions, the strictness trade-offs, and how the Go port differs from the
canonical TypeScript one. For hands-on material see
[`tutorial.md`](tutorial.md) and [`guide.md`](guide.md); for the exact API
see [`reference.md`](reference.md).

## The big idea: a grammar plugin, not a parser

The `tabnas` engine (`github.com/tabnas/parser/go`) ships **no grammar of
its own**. It is a configurable lexer plus a rule-driven parser, and it
does nothing useful until a *grammar plugin* tells it what tokens to
recognize and what rules to apply. `tabnasjson` is that plugin for
standard JSON.

So `tabnasjson` is two things bundled into the `Json` plugin:

1. **A lexer configuration** (`jsonOptions()`) that restricts the engine
   to JSON tokens: double-quoted strings, decimal numbers, the keywords,
   no comments, no bare text.
2. **A rule set** (`val` / `map` / `list` / `pair` / `elem`) registered
   via `RegisterJSONGrammar`, describing how those tokens nest into
   values.

`Parse` and `Make` are thin conveniences over "make an engine, install the
`Json` plugin."

## Where the grammar comes from

The rule set is not invented here — it is jsonic's **"Plain JSON"**
grammar. `jsonic` defines a pure-JSON core and then extends it for its
relaxed format (comments, unquoted keys, trailing commas, implicit
structures, single/backtick strings, path diving). This module takes that
pure core, installs it on its own, and clamps the lexer down to strict
JSON — and stops there.

That lineage is the key: **`tabnasjson` is the baseline that jsonic
relaxes.** Same `val` / `map` / `list` / `pair` / `elem` rules; jsonic
adds alternates and re-opens lexer options, `tabnasjson` keeps them
closed.

## The five rules

The grammar is five small rules, each a state machine with *open*
alternates (entering the rule) and *close* alternates (leaving it). The
start rule is `val`.

- **`val`** — a value is a `map` (sees `{`), a `list` (sees `[`), or a
  plain scalar token (`#VAL`). On close it resolves to its built child or
  its scalar.
- **`map`** — an object. Opens on `{`; closes immediately (`{}`) or matches
  `pair` rules; closes on `}`.
- **`list`** — an array. Opens on `[`; closes immediately (`[]`) or matches
  `elem` rules; closes on `]`.
- **`pair`** — one `"key": value` entry. Matches `#KEY #CL`, parses a
  `val`, loops on a comma.
- **`elem`** — one list element. Parses a `val`, loops on a comma.

This is why the module is a *foundation*: any plugin that wants
JSON-shaped structure can install these rules with `RegisterJSONGrammar`
and add its own alternates.

## How the value tree is built: the `$`-builtin actions

A grammar rule recognizes structure but does not, by itself, construct Go
values. This grammar has **no grammar-local closures at all**. Each
alternate names one of the engine's native-value **`$`-builtins**, and the
engine merges the real builder in at load time:

| Builtin | What it does |
|---|---|
| `@reset$` | Clear the parent-seeded node, so a scalar value does not inherit the parent container. |
| `@object$` | Allocate an empty object (a `map[string]any`, or a `MapRef` under `Info.Map`). |
| `@array$` | Allocate an empty array (a `[]any`, or a `ListRef` under `Info.List`). |
| `@key$` | Capture the matched key token into a scratch slot for the pending assignment. |
| `@setval$` | Assign the just-built child value into the object under the captured key. |
| `@push$` | Append the just-built child value to the array. |
| `@value$` | Resolve the rule's value: a built child wins, else the scalar token (a `Text` under `Info.Text`). |

These builders are **info-aware**. With `Info.Map` / `Info.List` /
`Info.Text` off (the strict-JSON default), they build plain
`map[string]any`, `[]any`, and primitive values. With them on, the *same*
builders allocate the engine's `MapRef` / `ListRef` / `Text` carriers and
record the container/quote metadata — no grammar change. The grammar spec
declares `V: 2`, the schema version of the builtins it binds to.

## What "strict" buys, and what it costs

The whole point is parity with `encoding/json`. The engine's lexer is
*lenient* by default — it will tokenize hex numbers, bare text, single
quotes. Strictness comes from a handful of options:

- `Number.Exclude` is a predicate that rejects any number token not
  matching `strictNumber`
  (`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`). That rejects `+1`,
  `.5`, `1.`, `01`, and `00`.
- `String.EscapeStrict: true`, plus dropping `v` / `'` / `` ` `` from the
  escape map and `AllowUnknown: false`, narrows escapes to exactly the
  JSON set. `EscapeStrict` disables the engine's structural `\xHH` and
  `\u{...}` escapes (plain `\uXXXX`, surrogate pairs included, stays).
- `Text.Lex`, `Comment.Lex`, `Map.Extend`, and `Lex.Empty` are off;
  `TokenSet["KEY"]` forces keys to be quoted strings only.

The trade-off is deliberate: this parser will **never** accept input
`encoding/json` rejects, and never reject input it accepts. For leniency,
reach for jsonic or layer your own options on top (the JSONC example in
[`guide.md`](guide.md)).

## TypeScript is canonical; Go tracks it

The TS implementation in `ts/src/json.ts` is the source of truth; this Go
port in `go/json.go` mirrors it line-for-line where it can — both use the
engine's declarative grammar spec, so the two grammars read almost
identically. Both suites run the same conformance fixtures in
`ts/test/spec/*.tsv` (`json-valid.tsv` = input → expected output,
`json-errors.tsv` = input → error code; the Go suite resolves them at
`../ts/test/spec`). The error **codes** are part of that shared contract:
both runtimes must reject the same input with the same code.

## Differences from the TS version

The behavior is identical (that is the parity contract), but the runtime
realities differ:

- **Error signaling.** Go `Parse` *returns* `(any, error)` where the error
  is a `*tabnas.TabnasError`; the TS `parse` *throws* a `TabnasError`.
  Inspect with `errors.As` / a type assertion in Go, `instanceof` in TS.
- **Error field names.** The Go error exposes `Row` / `Col` / `Pos`; the
  TS error exposes `lineNumber` / `columnNumber`. The `Code` values are
  identical (`unexpected`, `unterminated_string`, `invalid_unicode`).
- **Value types.** Go returns `map[string]any`, `[]any`, `float64`,
  `string`, `bool`, `nil` — the `encoding/json` set. TS returns plain
  objects (with a **`null` prototype**), arrays, `number`, `string`,
  `boolean`, `null`. Go has no prototype, so there is no
  prototype-pollution concern and no `null`-prototype caveat; key order is
  not preserved (Go maps are unordered), whereas TS objects keep insertion
  order.
- **Numbers are always `float64`.** Integers included (`1` → `float64(1)`),
  matching `encoding/json`. TS uses JavaScript's single `number` type.
- **Options shape.** Go configuration is the strongly-typed
  `tabnas.Options` struct with `*bool` pointer fields (hence the
  `tr := true; &tr` idiom); TS uses a plain nested options object. `Make`
  takes `extra ...tabnas.Options` (variadic) where TS `make` takes one
  optional options object.
- **Info carriers.** Under `Info` options Go yields concrete `MapRef` /
  `ListRef` / `Text` structs you type-assert; TS attaches a
  non-enumerable `__info__` marker to plain values (and boxes strings as
  `String` objects).
- **`Result.Fail` includes `NaN`.** Present only for TS parity; in Go the
  meaningful sentinel is `tabnas.Undefined` ("no value"), kept distinct
  from `nil` so JSON `null` stays valid.
- **Default-instance mechanism.** Go uses `sync.Once` to build the shared
  default parser; TS uses a lazily-assigned module variable
  (`??=`). Both reuse one engine and build a fresh context per parse, so
  both are safe to reuse.
- **The optional debug composition test** lives in a *separate* Go module
  (`go/debugtest/`) so the main module's `go test ./...` stays
  self-contained; in TS it is a normal test file gated on a dev
  dependency.

## The CLI

`tabnas-json` is a thin front end over `Parse`, the Go port of the TS
`json-cli`. The logic is split into a pure `run` (source string in, exit
code out, output via injected sink funcs) and a `runMain` that wires it to
args or stdin. Splitting it this way makes both halves testable in-process
without spawning a subprocess. It parses, then re-serializes with
`json.MarshalIndent(value, "", "  ")`, so its output is itself canonical
JSON — the same layout as the TS CLI's `JSON.stringify(value, null, 2)`.

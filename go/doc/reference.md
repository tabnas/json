# Reference: `tabnasjson` (Go)

The complete public API and CLI surface. Dry and exhaustive. For learning
see [`tutorial.md`](tutorial.md); for recipes see [`guide.md`](guide.md);
for design see [`concepts.md`](concepts.md).

- **Import path:** `github.com/tabnas/json/go`
- **Package name:** `tabnasjson`
- **Engine:** `github.com/tabnas/parser/go` (aliased `tabnas` below)
- **Go:** 1.24+

```go
import (
	tabnasjson "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)
```

## Exported symbols

| Symbol | Kind | Summary |
|---|---|---|
| `Parse` | func | Parse a string with the default engine. |
| `Make` | func | Build a configured parser instance. |
| `Json` | plugin func | Apply strict options + register the grammar on an engine. |
| `RegisterJSONGrammar` | func | Register only the rule set on an engine. |
| `Version` | const string | Module version (`"1.0.0"`). |

### `func Parse(src string) (any, error)`

Parses `src` as standard JSON using a single, lazily-created default
engine (created once via `sync.Once`; safe for concurrent use because each
parse builds its own context and only reads instance state). Returns the
parsed value, or a `*tabnas.TabnasError` on invalid input.

```go
v, err := tabnasjson.Parse(`{"a":1,"b":[2,3]}`)
// v == map[string]any{"a": 1, "b": []any{2, 3}}
```

Returned values use the same Go types as `encoding/json`:

| JSON | Go |
|---|---|
| object | `map[string]any` |
| array | `[]any` |
| string | `string` |
| number | `float64` (integers included: `1` → `float64(1)`) |
| `true` / `false` | `bool` |
| `null` | `nil` |

### `func Make(extra ...tabnas.Options) *tabnas.Tabnas`

Builds a fresh engine instance with the strict JSON options applied and
the grammar registered. Any `extra` options are applied with
`SetOptions` **after** the grammar exists, so they layer on top of the
strict configuration (and rule include/exclude filters operate on the
installed alternates). Returns a reusable, concurrency-safe `*tabnas.Tabnas`.

```go
tr := true
p := tabnasjson.Make(tabnas.Options{Info: &tabnas.InfoOptions{Map: &tr, List: &tr}})
```

(`Make` panics only if the fixed grammar spec is invalid — a programmer
error while editing the grammar, not reachable at runtime.)

### `func Json(j *tabnas.Tabnas, _ map[string]any) error`

The standard plugin form. Applies the strict JSON options
(`j.SetOptions(jsonOptions())`) and then calls `RegisterJSONGrammar(j)`.
Returns any error from grammar registration. Install it on a bare engine
with `Use`:

```go
j := tabnas.Make()
if err := j.Use(tabnasjson.Json); err != nil { /* ... */ }
v, err := j.Parse(`[1,2,3]`)
```

### `func RegisterJSONGrammar(j *tabnas.Tabnas) error`

Installs only the rule set (`val` / `map` / `list` / `pair` / `elem`) on
`j` via the engine's declarative grammar spec
(`j.Grammar(&tabnas.GrammarSpec{V: 2, Rule: rules})`). It does **not**
apply the strict lexer options, so use it to layer the JSON rules under
your own configuration. Returns any error from the grammar spec. The value
tree is built entirely by the engine's `$`-builtin actions; there are no
grammar-local closures.

### `const Version string`

The module version string (`"1.0.0"`), kept in sync with the TS package.

## Error type: `*tabnas.TabnasError`

Returned (not panicked) on invalid input. Match it with `errors.As` or a
type assertion. Relevant exported fields:

| Field | Type | Meaning |
|---|---|---|
| `Code` | `string` | Machine-readable error code (see below). |
| `Detail` | `string` | Human-readable detail message. |
| `Row` | `int` | 1-based line number. |
| `Col` | `int` | 1-based column number. |
| `Pos` | `int` | 0-based character position in source. |
| `Src` | `string` | Source fragment (token text) at the error. |

`Error()` returns a formatted, source-pointing message.

**Error codes** (the shared parity contract with the TS port):

| Code | When |
|---|---|
| `unexpected` | Any character/token no active rule alternative accepts — the catch-all (unquoted keys, trailing commas, comments, single quotes, bad numbers like `01`/`+1`/`.5`/`1.`, unknown escapes, empty input, trailing junk). |
| `unterminated_string` | A string literal with no closing quote (`"abc`). |
| `invalid_unicode` | A `\u` escape that is not four hex digits (`\uZ`, `\u{41}`). |

## Info carriers

When the engine's `Info` options are enabled via `Make`, parsed values are
wrapped in these engine types (instead of plain Go values):

| Type | Enabled by | Fields used here |
|---|---|---|
| `tabnas.MapRef` | `InfoOptions.Map` | `Val map[string]any`, `Implicit bool` |
| `tabnas.ListRef` | `InfoOptions.List` | `Val []any`, `Implicit bool` |
| `tabnas.Text` | `InfoOptions.Text` | `Str string`, `Quote string` |

For strict JSON every container is explicit, so `Implicit` is always
`false`; `Text.Quote` is always `"`.

## What is accepted

Exactly standard JSON (RFC 8259 / ECMA-404): objects with double-quoted
string keys, arrays, double-quoted strings (escapes `\" \\ \/ \b \f \n \r
\t` and `\uXXXX` including surrogate pairs), numbers (optional `-`,
no-leading-zero integer, optional fraction, optional `e`/`E` exponent),
`true`, `false`, `null`, and insignificant whitespace.

## What is rejected

Everything outside that grammar, matching `encoding/json`: comments,
trailing commas, unquoted keys, single-quoted/backtick strings, implicit
objects/arrays, hex/octal/binary numbers, leading zeros, leading `+`, bare
`.5`, trailing `1.`, non-standard escapes (`\x41`, `\u{41}`, `\v`, `\'`,
`` \` ``, `\q`), and empty or whitespace-only input.

## Strict options (internal)

`jsonOptions()` tightens the engine defaults to JSON-only. The
load-bearing settings (mirroring the TS `JSON_OPTIONS`):

| Option | Value | Effect |
|---|---|---|
| `Text.Lex` | `false` | No bare/unquoted text tokens. |
| `Number.Hex/Oct/Bin` | `false` | Decimal numbers only. |
| `Number.Sep` | `""` | No digit separators. |
| `Number.Exclude` | predicate over `strictNumber` | Rejects `+1`, `.5`, `1.`, `01`, `00`. |
| `String.Chars` | `` `"` `` | Double-quoted strings only. |
| `String.MultiChars` | `""` | No multiline string delimiters. |
| `String.AllowUnknown` | `false` | Reject unknown escapes (`\q`). |
| `String.EscapeStrict` | `true` | Disable `\xHH` and `\u{...}`. |
| `String.Escape` | `{v:"", "'":"", "`":""}` | Drop non-standard built-in escapes. |
| `Comment.Lex` | `false` | No comments. |
| `Map.Extend` | `false` | No trailing-comma map extension. |
| `Lex.Empty` | `false` | Reject empty input. |
| `Rule.Finish` | `false` | Require a complete parse. |
| `Result.Fail` | `[]any{tabnas.Undefined, math.NaN()}` | Treat "no value"/NaN as a parse failure. |
| `TokenSet["KEY"]` | `[]string{"#ST"}` | Keys must be quoted strings. |

`strictNumber` is `^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`.
`tabnas.Undefined` is the engine's "no value" sentinel — distinct from
`nil`, since JSON `null` parses to `nil` and must stay valid.

## Grammar rules

Installed by `RegisterJSONGrammar`. Each is a small open/close state
machine; the value tree is built by the engine's `$`-builtin actions (see
[`concepts.md`](concepts.md)).

| Rule | Role | Builders |
|---|---|---|
| `val` | a value: map, list, or scalar token | `@reset$`, `@value$` |
| `map` | an object `{ ... }` | `@object$` |
| `list` | an array `[ ... ]` | `@array$` |
| `pair` | a `"key": value` entry in a map | `@key$`, `@setval$` |
| `elem` | a value element in a list | `@push$` |

The start rule is `val`. A railroad diagram is in the TS docs at
[`../../ts/doc/grammar.svg`](../../ts/doc/grammar.svg) (ASCII:
[`../../ts/doc/grammar.txt`](../../ts/doc/grammar.txt)); the grammars are
identical across runtimes.

## CLI: `tabnas-json`

Command source at `go/cmd/tabnas-json/main.go`.

```
tabnas-json [json...]
```

- **With arguments:** the arguments are joined with a single space and
  parsed as one JSON source.
- **With no arguments:** all of stdin is read and parsed.

On success it writes the value re-serialized with
`json.MarshalIndent(value, "", "  ")` (2-space indent) plus a trailing
newline to stdout, and exits `0`. On a `*tabnas.TabnasError` it writes the
message to stderr and exits `1`.

```bash
go run ./go/cmd/tabnas-json '{"a":1,"b":[2,3]}'
echo '[1,2,3]' | go run ./go/cmd/tabnas-json
```

The command's logic is split into a pure `run(src, out, errOut)` (returns
the exit code and a non-parse error) and a `runMain(args, stdin, out,
errOut)` wiring function, both unexported and tested in-process by
`go/cmd/tabnas-json/main_test.go`.

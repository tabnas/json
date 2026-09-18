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
| `VERSION` | const string | Module version, always equal to `ts/package.json`. |

### `func Parse(src string) (any, error)`

Parses `src` as standard JSON using a single, lazily created default
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

(`Make` panics only if the fixed grammar spec is invalid, a programmer
error while editing the grammar, not reachable at runtime.)

### `func Json(j *tabnas.Tabnas, _ map[string]any) error`

The standard plugin form. Applies the strict JSON options
(`j.SetOptions(jsonOptions())`) and then calls
`RegisterJSONGrammar(j, GrammarOptions{ChainOff: true})`. Returns any
error from grammar registration. Install it on a bare engine
with `Use`:

```go
j := tabnas.Make()
if err := j.Use(tabnasjson.Json); err != nil { /* ... */ }
v, err := j.Parse(`[1,2,3]`)
```

### `func RegisterJSONGrammar(j *tabnas.Tabnas, extra ...GrammarOptions) error`

Installs only the rule set (`val` / `map` / `list` / `pair` / `elem`) on
`j` via the engine's declarative grammar spec
(`j.Grammar(&tabnas.GrammarSpec{V: 2, Rule: rules})`). It does **not**
apply the strict lexer options, so use it to layer the JSON rules under
your own configuration. Returns any error from the grammar spec. The value
tree is built entirely by the engine's `$`-builtin actions; there are no
grammar-local closures.

The variadic argument is the `Make(extra ...tabnas.Options)` shape used
elsewhere here: only the first is read, and no argument at all installs
the layerable grammar.

### `type GrammarOptions struct`

One field, `ChainOff bool`, whose zero value is the safe default.

`ChainOff: true` sets the engine's `push$.chain: false` on the two `elem`
close alternates, which stops `@push$` re-publishing the grown list back
along the `R: "elem"` replacement chain. It is a claim about the
**assembled** grammar: that no rule in it resolves `$prev` to read a rule
that `R: "elem"` replaced. The JSON core never does, but a plugin layering
its own alternates onto `elem` or `list` might, and only that plugin
knows. So this installer, the one built to be layered on, leaves the claim
unmade, and `Json`, the parser nobody has extended, is what turns it on.
Pass it yourself only if your own rules never read a replaced rule.

This is the port the flag matters in, both ways. A Go list is a slice
value, so each replaced rule holds its own header: the walk it skips is
quadratic in the element count, and a wrong claim silently gives a layered
plugin a stale list where TypeScript and Rust give it the live one. Those
two hand out one array object that every view already shares, so there the
same key is a no-op. The grammars are kept in step across the three ports,
so the flag is declared in all of them.

`push$.chain` arrives in the engine after the release `go.mod` currently
requires. An engine that predates it ignores the key rather than
rejecting it, so `ChainOff` is inert, not an error, until that
requirement moves.

### `const VERSION string`

The module version string. It always equals the TS package's
`ts/package.json` `"version"`; `TestVersionMatchesPackageJSON` fails the
build if the two ever drift.

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
| `unexpected` | Any character/token no active rule alternative accepts; the catch-all (unquoted keys, trailing commas, comments, single quotes, bad numbers like `01`/`+1`/`.5`/`1.`, unknown escapes, empty input, trailing junk). |
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

`jsonOptions()` tightens the engine defaults to JSON-only. The settings
that do the tightening (mirroring the TS `JSON_OPTIONS`):

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
`tabnas.Undefined` is the engine's "no value" sentinel, distinct from
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

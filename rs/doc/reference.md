# Reference: `tabnas-json` (Rust)

The complete public API. Dry and exhaustive. For learning see
[`tutorial.md`](tutorial.md); for recipes see [`guide.md`](guide.md); for
design see [`concepts.md`](concepts.md).

- **Package:** `tabnas-json`
- **Library:** `tabnas_json`
- **Engine:** `tabnas` (path dependency on `../../parser/rs`). Depend on
  it directly as well: a crate's dependencies are not passed on to its
  dependents, so `tabnas-json` alone does not put `tabnas` in your extern
  prelude. Only `JsonError` is re-exported.
- **Edition:** 2021
- **Minimum supported Rust version:** 1.85

## Exported items

| Item | Kind | Summary |
|---|---|---|
| `parse` | function | Parse a string with the shared default parser. |
| `make` | function | Build a fresh parser instance. |
| `json` | function | Install the strict options and the grammar on an engine. |
| `JsonError` | type alias | Re-export of `tabnas::TabnasError`. |
| `VERSION` | const | Crate version, always equal to `ts/package.json`. |

Everything else is private. There is no rules-only entry point matching
the Go `RegisterJSONGrammar`; see [Differences in the surface](#differences-in-the-surface).

### `pub fn parse(src: &str) -> Result<Value, JsonError>`

Parses `src` as standard JSON with a single default engine, built on
first use in a `OnceLock` and reused after that. Safe to call
concurrently: a parse borrows the instance immutably and builds its own
context, and `Tabnas` is `Send + Sync`.

```rust
let value = tabnas_json::parse(r#"{"a":1,"b":[2,3]}"#)?;
```

Returned values are `tabnas::Value`:

| JSON | `tabnas::Value` |
|---|---|
| object | `Object(IndexMap<String, Value>)`, in document order |
| array | `Array(Vec<Value>)` |
| string | `String(String)` |
| number | `Number(f64)`, integers included, so `1` is `Number(1.0)` |
| `true` and `false` | `Bool(bool)` |
| `null` | `Null` |

The `Undefined`, `Text`, `ListRef` and `MapRef` variants exist on the
enum but a strict-JSON parse never produces them, except under the info
options described below.

### `pub fn make() -> Tabnas`

Builds a fresh engine with the strict JSON options applied and the
grammar registered, by calling `json` rather than repeating the setup, so
the two construction paths cannot drift. The returned instance is
reusable, and `Send + Sync`.

```rust
let parser = tabnas_json::make();
let value = parser.parse("[1,2,3]")?;
```

It is infallible in its signature, and panics only if the fixed grammar
document is invalid. That is a bug in this crate rather than anything a
caller did, which is the same justification the Go `Make` gives for
panicking.

### `pub fn json(parser: &mut Tabnas) -> Result<(), GrammarError>`

The plugin form. Binds the number preflight hook by name, builds the
grammar document into a `GrammarSpec`, and installs it. Returns any error
from building or installing the spec. Install it on a bare engine:

```rust
use tabnas::Tabnas;

let mut parser = Tabnas::new();
tabnas_json::json(&mut parser)?;
let value = parser.parse("[1,2,3]")?;
```

Options applied with `set_options` after this call layer on top of the
strict configuration. Options applied before it are overwritten.

### `pub use tabnas::TabnasError as JsonError`

The error a failed parse produces, re-exported so callers need not name
the engine crate. Mirrors the TypeScript
`export { TabnasError as JsonError }`. Fields used here:

| Field | Type | Meaning |
|---|---|---|
| `code` | `String` | Machine-readable error code (see below). |
| `detail` | `String` | Human-readable detail message. |
| `row` | `usize` | 1-based line number. |
| `col` | `usize` | 1-based column number. |
| `pos` | `usize` | 0-based character position in source. |
| `src` | `String` | Source fragment (token text) at the error. |

It implements `std::error::Error` and `Display`; the `Display` form is a
formatted, source-pointing message.

**Error codes** (the shared parity contract with the TypeScript port):

| Code | When |
|---|---|
| `unexpected` | Any character or token no active rule alternative accepts; the catch-all (unquoted keys, trailing commas, comments, single quotes, bad numbers such as `01`, `+1`, `.5` and `1.`, unknown escapes, empty input, trailing junk). |
| `unterminated_string` | A string literal with no closing quote (`"abc`). |
| `cancel` | Nesting past the depth limit below. Rust only: neither other runtime limits depth, so no shared fixture pins this code. |
| `invalid_unicode` | A `\u` escape that is not four hex digits (`\uZ`, `\u{41}`). |

### `pub const VERSION: &str`

The crate version string. It always equals the TypeScript package's
`ts/package.json` `"version"`; `rs/tests/version_test.rs` fails the build
if `Cargo.toml`, this constant and `package.json` ever drift apart.

## Info carriers

When the engine's info options are enabled on an instance, parsed values
are wrapped in engine carriers instead of plain variants:

| Variant | Enabled by | Fields used here |
|---|---|---|
| `Value::MapRef` | `options.info.map` | `value: IndexMap<String, Value>`, `implicit: bool` |
| `Value::ListRef` | `options.info.list` | `value: Vec<Value>`, `implicit: bool` |
| `Value::Text` | `options.info.text` | `string: String`, `quote: String` |

For strict JSON every container is explicit, so `implicit` is always
`false`, and `quote` is always the double quote.

## What is accepted

Exactly standard JSON (RFC 8259 and ECMA-404): objects with
double-quoted string keys, arrays, double-quoted strings (escapes `\"`,
`\\`, `\/`, `\b`, `\f`, `\n`, `\r`, `\t` and `\uXXXX` including surrogate
pairs), numbers (optional `-`, no-leading-zero integer, optional
fraction, optional `e` or `E` exponent), `true`, `false`, `null`, and
insignificant whitespace.

Two exceptions, both following this runtime's platform parser rather than
TypeScript's. A number whose exponent puts it outside `f64` range, such
as `1e999`, is rejected rather than saturated to infinity; underflow to
zero, as in `1e-999`, is accepted. And nesting is limited to 127 levels.
See [`concepts.md`](concepts.md) for why parity is measured per runtime.

## The depth limit

`serde_json` accepts 127 levels of nested objects and arrays and refuses
one level deeper with "recursion limit exceeded". This crate does the
same, so `parse` answers a `cancel` error rather than a value:

```rust
let deep = "[".repeat(200) + &"]".repeat(200);
assert!(tabnas_json::parse(&deep).is_err());
```

The limit is not only about matching the platform. Without it, a
kilobyte of open brackets ends the **process** with a stack overflow
instead of returning an error, which no amount of care at the call site
can defend against. A parser reached with untrusted input has to answer,
not abort.

The boundary is measured against `serde_json` in the test suite rather
than copied from its source, so a future change there shows up as a
failure rather than as silent drift.

## What is rejected

Everything outside that grammar, matching `serde_json`: comments,
trailing commas, unquoted keys, single-quoted and backtick strings,
implicit objects and arrays, hex, octal and binary numbers, leading
zeros, a leading `+`, a bare `.5`, a trailing `1.`, non-standard escapes
(`\x41`, `\u{41}`, `\v`, `\'`, `` \` ``, `\q`), the bare words `NaN`,
`Infinity` and `undefined`, and empty or whitespace-only input.

## The grammar document

`json` builds one serialized document carrying both the options and the
rules, and installs it with `GrammarSpec::from_value`. The options travel
with the rules because `options.number.check` can only be bound by name
from outside the engine crate, and keeping them together leaves one
definition of strict JSON rather than two halves that can drift.

### Strict options

| Option | Value | Effect |
|---|---|---|
| `text.lex` | `false` | No bare or unquoted text tokens. |
| `number.hex` / `oct` / `bin` | `false` | Decimal numbers only. |
| `number.sep` | `null` | No digit separators. |
| `number.check` | `"json-strict-number"` | The preflight hook below. |
| `string.chars` | `"\""` | Double-quoted strings only. |
| `string.multiChars` | `""` | No multiline string delimiters. |
| `string.allowUnknown` | `false` | Reject unknown escapes such as `\q`. |
| `string.escapeStrict` | `true` | Disable `\xHH` and `\u{...}`. |
| `string.escape` | `{v: null, ': null, `: null}` | Delete the non-standard built-in escapes. |
| `comment.lex` | `false` | No comments. |
| `map.extend` | `false` | No trailing-comma map extension. |
| `lex.empty` | `false` | Reject empty input. |
| `rule.finish` | `false` | Require a complete parse. |
| `rule.include` | `"json"` | Keep only the `json`-tagged alternates. |
| `tokenSet.KEY` | `["#ST"]` | Keys must be quoted strings. |

Two entries differ from the other runtimes and are deliberate.

The escape entries are `null`, where `ts/src/json.ts` writes `''`. This
engine deletes an escape mapping on a null value and reads `""` as a real
mapping to the empty string, so `""` here would parse `"\v"` as `""`
instead of rejecting it.

`result.fail` is not set at all, where TypeScript and Go set it to the
undefined sentinel and `NaN`. Neither is reachable here: `text.lex` is
false, so the bare words `NaN`, `Infinity` and `undefined` are not tokens
in the first place, and `lex.empty` is false, so an empty document fails
before a result exists.

### The number preflight hook

`number.check` names a closure bound with `Tabnas::lex_check_ref`, which
is the only way to reach that option from outside the engine crate:
`LexCheck`'s constructors are crate-private. The hook takes the candidate
literal, up to the next whitespace or JSON structural character, and
answers `Skip` to reject it or `Continue` to allow it. It rejects:

1. Anything not matching
   `^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`, which is what
   turns away `+1`, `.5`, `1.`, `01` and `00`.
2. Anything matching that pattern whose `f64` value is not finite, which
   is what turns away `1e999`.

A literal that does not begin with a digit, `-`, `+` or `.` is passed
through untouched, because the hook runs wherever a number could be lexed
rather than only where one starts.

This is a hook rather than `options.number.exclude` because the
TypeScript exclude is a negative lookahead and the `regex` crate has no
lookaround, so the positive pattern plus an inversion is the only way to
express it. The Go port uses the same shape.

### Grammar rules

| Rule | Role | Builders |
|---|---|---|
| `val` | a value: map, list, or scalar token | `@reset$`, `@value$` |
| `map` | an object `{ }` | `@object$` |
| `list` | an array `[ ]` | `@array$` |
| `pair` | a `"key": value` entry in a map | `@key$`, `@setval$` |
| `elem` | a value element in a list | `@push$` |

The start rule is `val`. Every alternate is tagged `json`, which is what
makes `rule.include` meaningful when these options are applied over an
already-extended grammar. `ruleOrder` declares `val`, `map`, `list`,
`pair`, `elem`, so anything reading rule order reports this grammar as
written rather than alphabetically.

A railroad diagram is in the TypeScript docs at
[`../../ts/doc/grammar.svg`](../../ts/doc/grammar.svg) (ASCII:
[`../../ts/doc/grammar.txt`](../../ts/doc/grammar.txt)); the grammars are
identical across runtimes.

## Differences in the surface

Two things the other ports expose that this one does not.

**No rules-only entry point.** Go has `RegisterJSONGrammar`, which
installs the rules without the strict options. Here the rules and the
options are one document, and the document names the `number.check`
binding, so splitting them would produce a grammar referring to a hook
that had not been bound. Install `json` and relax what you need with
`set_options`, as in [`guide.md`](guide.md).

**No command-line tool.** TypeScript ships `json-cli` and Go ships
`tabnas-json`, both thin front ends over `parse` that re-serialize with a
2-space indent. The Rust port is a library only.

## Tests

| File | What it holds |
|---|---|
| `rs/tests/parity_test.rs` | The shared `test/spec/*.tsv` fixtures, with `serde_json` as a second opinion on every valid row. |
| `rs/tests/conformance_test.rs` | The pinned nst/JSONTestSuite corpus (318 cases), graded against `serde_json`, with the named divergence register. |
| `rs/tests/json_test.rs` | Behaviour the fixtures do not pin, including the two platform asymmetries and the shared default parser under concurrent callers. |
| `rs/tests/common/oracle.rs` | The one value comparator both graders use. |
| `rs/tests/version_test.rs` | `VERSION`, `Cargo.toml` and `ts/package.json` agree. |
| `rs/tests/common/spec.rs` | The fixture loader, matched to the `@tabnas/support` escape codec. |

Run them with `cargo test --all-targets` in `rs/`, or `make test-rs` from
the repository root. `ci/rust/run.sh` adds formatting, the lockfile
check, doctests and clippy at `-D warnings`.

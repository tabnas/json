# Concepts: how `tabnas-json` works and why (Rust)

Understanding-oriented. This explains the design: the engine
relationship, the grammar-plugin model, the role of the `$`-builtin
actions, the strictness trade-offs, and how the Rust port differs from
the canonical TypeScript one. For hands-on material see
[`tutorial.md`](tutorial.md) and [`guide.md`](guide.md); for the exact
API see [`reference.md`](reference.md).

## The big idea: a grammar plugin, not a parser

The `tabnas` engine ships **no grammar of its own**. It is a configurable
lexer plus a rule-driven parser, and it does nothing useful until a
grammar plugin tells it what tokens to recognize and what rules to apply.
`tabnas_json` is that plugin for standard JSON.

So `tabnas_json` is two things, and here they arrive as one document:

1. **A lexer configuration** restricting the engine to JSON tokens:
   double-quoted strings, decimal numbers, the keywords, no comments, no
   bare text.
2. **A rule set** (`val`, `map`, `list`, `pair`, `elem`) describing how
   those tokens nest into values.

`parse` and `make` are thin conveniences over "make an engine, install
the `json` plugin".

## Where the grammar comes from

The rule set is not invented here: it is jsonic's **"Plain JSON"**
grammar. `jsonic` defines a pure-JSON core and then extends it for its
relaxed format (comments, unquoted keys, trailing commas, implicit
structures, single and backtick strings, path diving). This crate takes
that pure core, installs it on its own, clamps the lexer down to strict
JSON, and stops there.

That lineage is the point: **`tabnas_json` is the baseline that jsonic
relaxes.** Same five rules; jsonic adds alternates and re-opens lexer
options, `tabnas_json` keeps them closed.

## The five rules

The grammar is five small rules, each a state machine with *open*
alternates (entering the rule) and *close* alternates (leaving it). The
start rule is `val`.

- **`val`**. A value is a `map` (sees `{`), a `list` (sees `[`), or a
  plain scalar token (`#VAL`). On close it resolves to its built child or
  its scalar.
- **`map`**. An object. Opens on `{`; closes immediately for `{}` or
  matches `pair` rules; closes on `}`.
- **`list`**. An array. Opens on `[`; closes immediately for `[]` or
  matches `elem` rules; closes on `]`.
- **`pair`**. One `"key": value` entry. Matches `#KEY #CL`, parses a
  `val`, loops on a comma.
- **`elem`**. One list element. Parses a `val`, loops on a comma.

This is why the crate is a *foundation*: any plugin that wants
JSON-shaped structure can install these rules and add its own alternates
on top.

## How the value tree is built: the `$`-builtin actions

A grammar rule recognizes structure but does not, by itself, construct
values. This grammar has **no grammar-local closures at all**. Each
alternate names one of the engine's native-value `$`-builtins, and the
engine merges the real builder in at load time:

| Builtin | What it does |
|---|---|
| `@reset$` | Clear the parent-seeded node, so a scalar value does not inherit the parent container. |
| `@object$` | Allocate an empty object (an `IndexMap`, or a `MapRef` under `info.map`). |
| `@array$` | Allocate an empty array (a `Vec`, or a `ListRef` under `info.list`). |
| `@key$` | Capture the matched key token into a scratch slot for the pending assignment. |
| `@setval$` | Assign the just-built child value into the object under the captured key. |
| `@push$` | Append the just-built child value to the array. |
| `@value$` | Resolve the rule's value: a built child wins, else the scalar token (a `Text` under `info.text`). |

These builders are **info-aware**. With the info options off, which is
the strict-JSON default, they build plain `Object`, `Array` and scalar
values. With them on, the same builders allocate the engine's `MapRef`,
`ListRef` and `Text` carriers and record the container and quote
metadata, with no grammar change. The document declares `"v": 2`, the
schema version of the builtins it binds to.

## Why the options travel with the rules

The other two ports keep their lexer options and their rule registration
in separate functions, and Go exposes the second one as
`RegisterJSONGrammar`. This port keeps them in one document, for a
reason rather than for tidiness.

`options.number.check` holds the preflight hook that decides what counts
as a standard JSON number. It can only be bound **by name** from outside
the engine crate: `LexCheck`'s constructors are crate-private, and
`Tabnas::lex_check_ref` exists for exactly this. So the options have to
name a binding this crate made, and a rules-only entry point would hand
back a grammar referring to a hook nobody had bound. Keeping the two
halves together also means there is one definition of strict JSON here
rather than two that can drift.

## What "strict" buys, and what it costs

The aim is parity with `serde_json`. The engine's lexer is *lenient* by
default: left alone it will tokenize hex numbers, bare text and single
quotes. Strictness comes from a handful of options:

- `number.check` rejects any number token outside
  `^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`, which turns away
  `+1`, `.5`, `1.`, `01` and `00`.
- `string.escapeStrict`, plus deleting `v`, `'` and `` ` `` from the
  escape map and setting `allowUnknown` to false, narrows escapes to
  exactly the JSON set. `escapeStrict` disables the engine's structural
  `\xHH` and `\u{...}` escapes; plain `\uXXXX`, surrogate pairs included,
  stays.
- `text.lex`, `comment.lex`, `map.extend` and `lex.empty` are off, and
  `tokenSet.KEY` forces keys to be quoted strings only.

The trade-off is deliberate. On every input RFC 8259 rules on, this
parser accepts what `serde_json` accepts and rejects what it rejects. The
exceptions are the eleven cases the standard leaves implementation-
defined, registered and explained below. For leniency, reach for jsonic
or layer your own options on top, as the JSONC recipe in
[`guide.md`](guide.md) does.

## TypeScript is canonical; Rust tracks it

The TypeScript implementation in `ts/src/json.ts` is the source of truth.
This port in `rs/src/lib.rs` mirrors it where it can: both express the
grammar as a serialized document, so the two rule sets read almost
identically. All three suites run the same conformance fixtures in
`test/spec/*.tsv` (`valid.tsv` maps input to expected output,
`errors.tsv` maps input to an error code). The error **codes** are part
of that shared contract: every runtime must reject the same input with
the same code.

The fixtures have one subtlety worth stating outright. The **input
column is escape-decoded and the expected column is not**. Decoding both
collapses one layer twice and changes what a row asserts, which is why
`rs/tests/common/spec.rs` follows the codec in `@tabnas/support` rather
than carrying a second copy of it.

## Parity is measured per runtime

Each port is held to **its own platform**, not to TypeScript's. That is
what a JSON parser is for: a caller reaching for this crate wants what
`serde_json` would have given them.

Almost always the platforms agree and the distinction is invisible. Two
inputs make it visible.

`1e999` is syntactically valid JSON, and the three platform parsers
disagree about it:

| Platform | `1e999` | `1e-999` | 200 levels of nesting |
|---|---|---|---|
| `JSON.parse` | infinity | `0` | accepted |
| `encoding/json` | error | `0` | accepted |
| `serde_json` | error | `0` | error |

So TypeScript accepts the exponent and both ports reject it, and only
Rust limits nesting. `rs/tests/json_test.rs` pins both by asserting
**both** that this crate rejects the input and that `serde_json` does. If
a future `serde_json` starts accepting either, the test goes red and the
divergence is revisited rather than going wrong unnoticed.

The nesting limit carries a second justification the exponent does not.
Without it, a kilobyte of open brackets ends the process with a stack
overflow instead of returning an error. A library that parses input its
caller did not write has to answer rather than abort, so the limit would
be worth having even if `serde_json` had none.

The same test file carries the parity runner's second opinion: every
valid fixture row is parsed by `serde_json` as well, and the two results
must agree. That is what makes the asymmetries above testable rather than
merely asserted, and it is the same technique `go/parity_test.go` uses
against `encoding/json`.

## The external corpus, and what it found

`rs/tests/conformance_test.rs` grades the pinned
[nst/JSONTestSuite](https://github.com/nst/JSONTestSuite) corpus, the
same 318 cases the TypeScript and Go suites grade, fetching it on first
use. Case names carry the contract: `y_` must be accepted, with the value
the platform parser produces; `n_` must be rejected, with a code; and
`i_` is implementation-defined, where the promise is agreement with the
platform.

**All 283 mandatory cases pass.** The corpus is what exercises the depth
limit above: `i_structure_500_nested_arrays` is one kilobyte of
brackets.

Eleven of the 35 implementation-defined cases differ from `serde_json`,
and each is named in the test with its reason rather than skipped as a
class. Ten are lone surrogates in a `\u` escape, which `serde_json`
rejects and this parser turns into U+FFFD, as `encoding/json` does;
changing that would make this runtime's string decoding differ from the
other two, which is a decision for the engine rather than for a grammar
plugin. The eleventh is a 48-digit integer where `serde_json` lands a
unit in the last place away from what the Rust standard library returns,
so here the engine is the accurate one and matching the oracle would mean
being deliberately less so.

The register is asserted in both directions. A case that leaves the list
by starting to agree fails as a stale entry, and one that joins it
without being written down fails as a regression, so the count cannot
drift on its own.

Rust also meets the corpus at a boundary the other two do not have.
Twenty-five cases are not valid UTF-8, and `parse` takes a `&str`, which
cannot hold them: such a source cannot be built into an argument at all,
so a caller has no way to submit it. The grader reports that as the rejection
it is, and holds `serde_json::from_slice`, which also refuses invalid
UTF-8, to the same bytes.

## Differences from the TypeScript version

Three behaviours differ, each because this port follows its own platform
rather than TypeScript's: the number range and the nesting limit above,
and the key ordering below. Everything else is the parity contract, and
what changes is the runtime realities:

- **Error signaling.** Rust `parse` *returns* `Result<Value, JsonError>`;
  the TypeScript `parse` *throws* a `TabnasError`. `JsonError` is the
  engine's error type re-exported, so callers need not name the engine
  crate.
- **Error field names.** The Rust error exposes `row`, `col` and `pos`;
  the TypeScript error exposes `lineNumber` and `columnNumber`. The
  `code` values are identical.
- **Value types.** Rust returns the engine's `Value` enum, so reading a
  parsed document means matching on a variant. TypeScript returns plain
  objects (with a **null prototype**), arrays, `number`, `string`,
  `boolean` and `null`. Rust has no prototype, so there is no
  prototype-pollution concern and no null-prototype caveat.
- **Key order is document order**, because the `Object` variant holds an
  `IndexMap`, which is what `serde_json` gives. Go differs: a map is
  unordered. TypeScript differs in one case, because a JavaScript object
  enumerates integer-like keys first in ascending numeric order, so
  `{"2":"a","1":"b"}` reads back as `1`, `2` there and as `2`, `1` here.
  Non-index keys keep insertion order in both.
- **Numbers are always `f64`.** Integers included, so `1` becomes
  `Number(1.0)`, matching `serde_json`. TypeScript uses JavaScript's
  single `number` type.
- **Options shape.** Rust configuration is the typed `Options` struct
  with plain fields, mutated through a closure passed to `set_options`.
  TypeScript uses a plain nested options object, and Go uses a struct of
  pointer fields, hence its `tr := true` idiom. Neither `make` nor `json`
  takes options here: apply them afterwards.
- **Strictness is a `check` hook, not `number.exclude`.** The TypeScript
  exclude is a negative lookahead, and the `regex` crate has no
  lookaround at all, so the positive pattern plus an inversion is the
  only way to express it. Go reached the same shape for its own reasons.
- **Deleting an escape.** TypeScript removes a mapping by setting it
  falsy; this engine deletes on `null` and treats `""` as a real mapping
  to the empty string. Same intent, different idiom, and only the `null`
  form rejects `"\v"` rather than reading it as an empty string.
- **`result.fail` is unset.** TypeScript and Go set it to the undefined
  sentinel and `NaN`. Neither is reachable here, because `text.lex` is
  false and the bare words are therefore not tokens at all.
- **Nesting is capped at 127 levels**, through the engine's parse budget.
  Neither other runtime caps it, because neither other platform parser
  does. The rejection carries the engine's `cancel` code, which is
  therefore a Rust-only code with no shared fixture behind it.
- **Default-instance mechanism.** Rust uses a `OnceLock`; Go uses
  `sync.Once`; TypeScript uses a lazily assigned module variable. All
  three reuse one engine and build a fresh context per parse, so all
  three are safe to reuse.
- **No command-line tool.** TypeScript ships `json-cli` and Go ships
  `tabnas-json`. The Rust port is a library only.

## Clippy and the large error

The crate allows `clippy::result_large_err` at its root. The engine's
error carries a code, a position, a hint and a formatted report, so it is
large by design, and the engine allows the same lint at its own root for
the same reason. Boxing the error here would make `parse` return a
different shape from `Tabnas::parse` and from both other ports, which is
a worse trade than the lint.

## How the crate is built and checked

The engine is a path dependency on a sibling checkout, so there is
nothing to fetch and `rs/Cargo.lock` records a resolution that includes
an unpublished crate. `ci/rust/run.sh` is the gate: it pins the minimum
supported Rust version from `Cargo.toml`, checks that the lockfile does
not change during the run (exempting the engine's own version, which
moves with the sibling), and runs formatting, the build, the tests, the
doctests and clippy at `-D warnings`. The same script runs in
`tabnas/debug` and `tabnas/directive`, byte for byte, so a fix to one is
a fix to all three.

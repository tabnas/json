# Agents Guide — rs/

The Rust port of the canonical TypeScript in [`../ts`](../ts). Read
[`../AGENTS.md`](../AGENTS.md) first: it holds the cross-runtime rules,
and this file only covers what is specific to this crate.

## Layout

| Path | |
|---|---|
| `src/lib.rs` | the whole port: options, grammar document, plugin, `make`, `parse` |
| `tests/parity_test.rs` | the shared `../test/spec/*.tsv` fixtures, plus the serde_json oracle |
| `tests/json_test.rs` | in-language behaviour and the deliberate asymmetries |
| `tests/version_test.rs` | the version sites must agree |
| `tests/common/spec.rs` | the fixture loader |

## The grammar travels as one document

Options and rules are one serialized `GrammarSpec`, not a typed `Options`
struct plus a separate grammar call. That is not a style choice:
`options.number.check` can only be bound by NAME from outside the engine
crate (`LexCheck`'s constructors are `pub(crate)`, and
`Tabnas::lex_check_ref` exists for exactly this), and the strictness hook
needs it. Keeping the options beside the rules also means there is one
definition of "strict JSON" rather than two halves that can drift.

## Three things that do not port directly

1. **`number.exclude`** is a negative lookahead in TypeScript. The
   `regex` crate has no lookaround, so the positive pattern plus an
   inversion lives in the `check` hook instead — the shape Go uses.
2. **`escape: { v: '' }`** deletes an escape in TypeScript. This engine
   treats `""` as a real mapping to the empty string and deletes only on
   `null`, so the entries here are `null`. With `""` the non-standard
   escapes parse as `""` instead of being rejected, silently, on five
   spec rows.
3. **Out-of-range exponents** are rejected, matching `serde_json` and so
   matching Go rather than TypeScript. `tests/json_test.rs` pins it AND
   asserts the oracle agrees, so the divergence cannot quietly become
   wrong.

## The fixture loader can drift

`@tabnas/support` has no Rust half, so `tests/common/spec.rs` is a third
independent implementation of one format. Two things about it are
load-bearing:

- The **input** column is escape-decoded and the **expected** column is
  not. That asymmetry is the canonical runner's, and decoding both
  collapses one layer twice: it changed what 22 rows asserted before it
  was matched to `@tabnas/support`'s `escape.ts`.
- A comment line is a `#` line **with no tab**; a data row always has one.

Change the fixture format and you change three loaders, not two.

## Running it

`make test-rs` is the fast loop. `ci/rust/run.sh` is the full gate and is
what CI would run: it adds `cargo fmt --check`, a build, doctests, the
lockfile check and the MSRV pin. The engine must be a sibling checkout at
`../../parser`.

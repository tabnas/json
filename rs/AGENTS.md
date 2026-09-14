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
| `doc/*.md` | the four reader-facing Diátaxis pages, gated by the prose gate |
| `README.md` | the crate front page, also gated |

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

## The default parser is shared

`parse` builds its engine once in a `OnceLock` and reuses it, matching
`sync.Once` in Go and the lazily assigned module variable in TS. That is
sound because `Tabnas::parse` takes `&self` and builds a fresh context
per call, and `Tabnas` is `Send + Sync`. The compiler pins the sharing;
what it does not pin is that the shared engine keeps no state between
parses, so `the_shared_default_parser_takes_concurrent_callers` in
`tests/json_test.rs` interleaves failing parses with succeeding ones
across threads. Do not "optimise" `parse` back to `make().parse(src)`:
that rebuilds the whole grammar per call.

## The docs are gated

`doc/{tutorial,guide,reference,concepts}.md` and `README.md` are in the
published set declared by `ts/scripts/gated-docs.cjs`, so both halves of
the prose gate cover them. Two consequences when editing them:

- The rules in [`../docs/STYLE-GUIDE.md`](../docs/STYLE-GUIDE.md) apply:
  no em dashes in prose, no first person singular anywhere, first person
  plural in the tutorial only, the banned-phrase list, and **a published
  page never cites an internal one** (no links to any `AGENTS.md`, and no
  project history about what a bug used to do).
- A Rust term Vale's dictionary lacks goes in
  `.vale/styles/config/vocabularies/Tabnas/accept.txt`, one word per
  line, never as a suffix pattern. After any edit re-measure with
  `node ts/scripts/vale-counts.cjs --write`, or the recorded counts in
  `.vale.ini` and the style guide go stale and CI fails on the drift.

## Running it

`make test-rs` is the fast loop. `ci/rust/run.sh` is the full gate and is
what CI would run: it adds `cargo fmt --check`, a build, doctests, the
lockfile check and the MSRV pin. The engine must be a sibling checkout at
`../../parser`.

For the docs, run both halves from the repo root:

```bash
node --test ts/test/docs.test.js
vale --minAlertLevel=error $(node ts/scripts/gated-docs.cjs)
node ts/scripts/vale-counts.cjs
```

Vale must be the version pinned in `ci/workflows/docs.yml`; the counts
script reads that pin and measuring with another binary rewrites the
record to numbers CI will not reproduce.

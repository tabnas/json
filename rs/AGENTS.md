# Agents Guide — rs/

The Rust port of the canonical TypeScript in [`../ts`](../ts). Read
[`../AGENTS.md`](../AGENTS.md) first: it holds the cross-runtime rules,
and this file only covers what is specific to this crate.

## Layout

| Path | |
|---|---|
| `src/lib.rs` | the whole port: options, grammar document, plugin, `make`, `parse`, and the unit tests that read the document itself |
| `tests/parity_test.rs` | the shared `../test/spec/*.tsv` fixtures, plus the serde_json oracle |
| `tests/conformance_test.rs` | the external nst/JSONTestSuite corpus, and the divergence register |
| `tests/common/oracle.rs` | one value comparator, shared by both of those |
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

## The depth limit is a crash fix as well as a parity one

`json()` binds `options.parse.budget` to a check that counts open `map`
and `list` rules and refuses past `DEPTH_LIMIT` (127, which is what
serde_json accepts). Two independent reasons, and BOTH have to stay true
of any replacement:

1. **Parity.** serde_json refuses the 128th level; `JSON.parse` and
   `encoding/json` go far deeper, so TS and Go set no limit. Rule 4 is
   per-runtime, so Rust follows its own platform.
2. **It is the difference between an error and an abort.** Without it,
   1 KB of open brackets ends the PROCESS with a stack overflow. That is
   not a tuning question for a parser reached with untrusted input.

Three details that look arbitrary and are not:

- The predicate is `<=`, and the count includes the rule the loop is
  inside. The engine hands the budget check that rule separately, as
  `context.rule`, and `rule_stack` holds only its ancestors; a container
  is open from the moment it is the current rule, so it is counted. When
  only the ancestors were counted the boundary depended on what the
  innermost container held: `[]` nested 127 deep parsed while `[1]`
  nested 127 deep, and 127 nested objects, answered `cancel`, which
  serde_json accepts. The suite measures every shape against the oracle.
- Depth is counted from RULE NAMES, not `rule_stack.len()`. The stack
  holds about three rules per level, so a length limit would encode that
  ratio and shift the first time the grammar gains an alternate.
- `parse_budget` is called AFTER `grammar()`. An options pass that does
  not mention `parse.budget` is not required to preserve one set before
  it.

The rejection carries the engine's `cancel` code. That is Rust-only:
there is no shared fixture for it, because the other two runtimes have
nothing to agree with.

## Two divergences no shared fixture can hold

Both are per-runtime parity (rule 4), and both are invisible to
`test/spec/*.tsv` because a fixture row has ONE expected column and these
runtimes legitimately produce different text.

- **Integer-like object keys.** A JavaScript object enumerates them
  first, in ascending numeric order, so TS reads `{"2":"a","1":"b"}` back
  as `1`, `2`. An `IndexMap` keeps document order, as `serde_json` does.
  Pinned by `integer_like_keys_keep_document_order_like_its_platform_oracle`.
- **The `cancel` code.** The depth budget's rejection. Neither other
  runtime limits depth, so it must never reach a shared fixture.

Both were found by review, not by a test, and the docs asserted the
opposite of the first until then. When adding a fixture, ask whether the
row's expected text is the same in all three runtimes before assuming it
is shareable.

## The conformance grader has a divergence register

`tests/conformance_test.rs` grades the pinned nst/JSONTestSuite corpus,
fetching it through `corpus()` when absent, the way Go's `TestMain` does.
All 95 `y_` and all 188 `n_` cases agree with serde_json. Eleven of the
35 `i_` cases do not, and each is named in `KNOWN_DIVERGENCES` with its
reason (ten lone-surrogate cases, one 48-digit integer where serde_json
is 1 ulp off `str::parse::<f64>` and the engine is the accurate one).

**The register is asserted in both directions.** A listed case that
starts agreeing fails as a stale entry; an unlisted case that starts
differing fails as a regression. Do not "fix" a red one by adding it to
the list without a reason that survives reading.

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
what CI would run: it adds `cargo fmt --check`, a build, doctests, clippy
at `-D warnings`, a rustdoc build (`cargo doc --no-deps` under
`RUSTDOCFLAGS=-D warnings`, which is the only arm that sees a broken or
ambiguous intra-doc link), the lockfile check and the MSRV pin. The
doctests include `README.md` through a `#[cfg(doctest)]` include in
`src/lib.rs`, so every `rust` fence in the README is compiled and run as
written: keep each one a complete `fn main` example. The engine must be a
sibling checkout at `../../parser`.

For the docs, run both halves from the repo root:

```bash
node --test ts/test/docs.test.js
vale --minAlertLevel=error $(node ts/scripts/gated-docs.cjs)
node ts/scripts/vale-counts.cjs
```

Vale must be the version pinned in `.github/workflows/docs.yml`; the
counts script reads that pin and measuring with another binary rewrites
the record to numbers CI will not reproduce.

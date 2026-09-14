# tabnas-json (Rust)

Standard JSON grammar plugin for the
[`tabnas`](https://github.com/tabnas/parser) parsing engine, crate
`tabnas_json`.

The engine ships no grammar of its own; this crate supplies the strict,
standard-JSON one. The rule set (`val` / `map` / `list` / `pair` /
`elem`) is jsonic's "Plain JSON" grammar, installed on its own with the
lexer restricted to strict JSON: double-quoted strings, plain decimal
numbers, quoted keys, no comments, no trailing commas.

This is the Rust port of the canonical TypeScript implementation in
[`../ts`](../ts); the TypeScript version is authoritative and this crate
tracks it.

## Documentation

Full [Diátaxis](https://diataxis.fr) docs:

- [`doc/tutorial.md`](doc/tutorial.md). Learn it step by step.
- [`doc/guide.md`](doc/guide.md). Task-focused recipes.
- [`doc/reference.md`](doc/reference.md). The exact API surface.
- [`doc/concepts.md`](doc/concepts.md). How it works, including the
  differences from the TypeScript version.

TypeScript is canonical; its docs are in [`../ts/doc/`](../ts/doc/), and
the Go port has the equivalent set in [`../go/doc/`](../go/doc/).

## Use

```rust
let value = tabnas_json::parse(r#"{"a":[1,2]}"#)?;
```

Or build an instance and reuse it:

```rust
let parser = tabnas_json::make();
let value = parser.parse("[1,2,3]")?;
```

To layer another grammar on the JSON core, install the plugin on your own
instance and add rules on top of the shared `val` / `map` / `list` /
`pair` / `elem`:

```rust
let mut parser = tabnas::Tabnas::new();
tabnas_json::json(&mut parser)?;
```

## Install

The `tabnas` crate is not published to a registry, so the engine is
consumed as a **sibling checkout**, the standard tabnas development
model. Clone `https://github.com/tabnas/parser` next to this repository
and point at it:

```toml
[dependencies]
tabnas-json = { path = "../json/rs" }
```

## Differences from the canonical TypeScript

Two, both deliberate:

- **Out-of-range exponents are rejected.** `1e999` is syntactically valid
  JSON, and the platform parsers disagree about it: `JSON.parse`
  saturates to `Infinity`, while `serde_json` and `encoding/json` both
  error. This package is held to per-runtime parity, so Rust rejects with
  Go rather than accepting with TypeScript. Underflow (`1e-999` to `0`)
  is accepted, as it is in Go.
- **Strictness is a `check` hook, not `number.exclude`.** The TypeScript
  exclude is a negative lookahead, and the `regex` crate has no
  lookaround, so the positive pattern plus an inversion is the only way
  to express it here. That is the shape the Go port already uses.

## Build and test

The engine is a path dependency on the sibling checkout, so there is
nothing to fetch:

```bash
cargo test --all-targets
```

Or, from the repository root, `make test-rs`. For what CI would say,
including formatting and the lockfile check, run `ci/rust/run.sh`.

The suite runs the shared `../test/spec/*.tsv` conformance fixtures, the
same files the TypeScript and Go suites run, and additionally
cross-checks every valid row against `serde_json`, this runtime's
platform oracle, the way the Go runner checks against `encoding/json`.

## License

MIT.

# Tutorial: parsing your first JSON with `tabnas-json` (Rust)

A learning-oriented walkthrough. You start with nothing and end with a
working Rust program that parses JSON, handles an error, and extends the
parser. Follow it top to bottom.

This is the Rust port of [`@tabnas/json`](../../ts/). For a reference of
the API read [`reference.md`](reference.md); for recipes read
[`guide.md`](guide.md); for design read [`concepts.md`](concepts.md).

The package is `tabnas-json` and the library is `tabnas_json`.

## What you are building

`tabnas_json` is a standard JSON parser: it accepts the grammar of
RFC 8259 and ECMA-404 and nothing more. On everything those standards
make mandatory it agrees with `serde_json` exactly; on the handful of
cases they leave implementation-defined it differs in eleven documented
places, listed in `rs/tests/conformance_test.rs` and explained in
[`concepts.md`](concepts.md). By the end you will have used it to turn
JSON text into values, caught a parse error, and built a
JSON-with-comments parser on top of it.

## Step 1: Add the dependency

The `tabnas` engine is not published to a registry yet, so both it and
this crate are consumed as sibling checkouts, the standard tabnas
development model. Clone `https://github.com/tabnas/parser` and
`https://github.com/tabnas/json` next to each other, then point at the
crate by path:

```toml
[dependencies]
tabnas-json = { path = "../json/rs" }
tabnas = { path = "../parser/rs" }
```

**Both entries are needed.** A crate's dependencies are not passed on to
its dependents, so `tabnas-json` alone does not put `tabnas` in your
extern prelude, and the `use tabnas::...` lines below would not resolve.
The crate re-exports only `JsonError`, for the one type you cannot avoid
touching. Go asks for the same two imports for the same reason.

## Step 2: Parse a value

The whole library is reachable through one function: `parse`. Give it a
JSON string; get back a value or an error.

```rust
fn main() {
    let value = tabnas_json::parse("42").expect("valid JSON");
    println!("{value:?}"); // Number(42.0)
}
```

Parsed values are the engine's `tabnas::Value`, an enum with one variant
per JSON shape: `Null`, `Bool`, `Number`, `String`, `Array`, `Object`.
Every number is an `f64`, integers included, which is why `42` prints as
`Number(42.0)`.

## Step 3: Parse objects and arrays

The same `parse` handles structured data, nested as deeply as a real
document needs.

```rust
use tabnas::Value;

let object = tabnas_json::parse(r#"{"a":1,"b":2}"#)?;
// Value::Object with keys "a" and "b"

let array = tabnas_json::parse("[1, 2, 3]")?;
// Value::Array of three numbers

let nested = tabnas_json::parse(r#"{"a": {"b": [true, null]}}"#)?;
// Value::Object -> Value::Object -> Value::Array

if let Value::Object(fields) = &object {
    println!("{:?}", fields["a"]); // Number(1.0)
}
```

Raw string literals (`r#"..."#`) keep the double quotes readable, and
insignificant whitespace between tokens is ignored, as in standard JSON.

Nesting is allowed up to 127 levels, which is where `serde_json` stops
too. Step 4 says what happens past that.

An `Object` holds an `IndexMap`, so the keys come back in the order the
document wrote them. That is a difference from the Go port, where a map
is unordered.

## Step 4: See what gets rejected

This parser is strict. Anything `serde_json` would reject, it rejects
too. Each of these returns an error:

```rust
tabnas_json::parse("{a:1}");   // unquoted key
tabnas_json::parse("[1,2,]");  // trailing comma
tabnas_json::parse("'x'");     // single-quoted string
tabnas_json::parse("01");      // leading zero
tabnas_json::parse("1 // hi"); // comment
```

Two more are rejected because `serde_json` rejects them, even though
`JSON.parse` accepts both: a number whose exponent is out of `f64` range,
such as `1e999`, and nesting deeper than 127 levels. The second also
keeps a deeply nested document from ending the process rather than
returning an error.

That strictness is the reason the crate exists: `tabnas_json` is the
baseline that relaxed variants extend.

## Step 5: Handle a parse error

Invalid input returns `Err(JsonError)`, which is the engine's
`TabnasError` re-exported under a local name. Read its structured fields
rather than the message text:

```rust
match tabnas_json::parse("{a:1}") {
    Ok(value) => println!("{value:?}"),
    Err(err) => println!("{} at {}:{}", err.code, err.row, err.col),
    // unexpected at 1:2
}
```

`code` is the machine-readable classification, and `row` and `col` are
1-based. The `Display` form, which is what `{err}` and `expect` print, is
a human-readable message pointing at the offending source.

## Step 6: Build your own parser instance

`parse` uses one shared engine, built on first use and reused after that.
When you want to customize the parser, build your own instance with
`make`:

```rust
let parser = tabnas_json::make();
let value = parser.parse("[1,2]")?;
```

An instance is reusable and safe to share between threads: build it once,
call `parse` on it many times.

## Step 7: Extend the grammar (JSON-with-comments)

Here is the foundation idea in action. The `json` plugin installs the
JSON grammar onto a bare engine, and `set_options` layers extra options
on top. Turning comment lexing back on turns strict JSON into JSONC:

```rust
use tabnas::{Options, Tabnas};

let mut jsonc = Tabnas::new();
tabnas_json::json(&mut jsonc)?;
jsonc.set_options(|options: &mut Options| {
    options.comment.lex = true;
})?;

jsonc.parse(r#"{"a":1} // a note"#)?;   // Object {"a": 1}
jsonc.parse(r#"{"a":/* inline */2}"#)?; // Object {"a": 2}
```

The top-level `parse` still rejects those comments; you extended a new
instance, not the shared one.

Note the order. The options are applied after the grammar is installed,
so they layer over the strict configuration rather than being overwritten
by it. Reversing the two lines gives you strict JSON again.

## Step 8: Run the tests

There is no separate fetch step, so the suite runs from a fresh checkout:

```bash
cd rs && cargo test --all-targets
```

That runs the shared conformance fixtures in `test/spec/*.tsv`, the same
files the TypeScript and Go suites run, and cross-checks every valid row
against `serde_json`. It also grades the external nst/JSONTestSuite
corpus, downloading it at a pinned commit the first time. For what CI
would say, including formatting, the lockfile check and clippy, run
`ci/rust/run.sh` from the repository root.

## Where to go next

- [`guide.md`](guide.md). Task-focused recipes.
- [`reference.md`](reference.md). The exact API surface.
- [`concepts.md`](concepts.md). How the grammar-plugin model works, plus
  the differences from the TypeScript version.

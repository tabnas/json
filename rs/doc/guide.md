# How-to guide: `tabnas-json` recipes (Rust)

Task-oriented recipes for real problems. Each section is self-contained.
For a guided introduction read [`tutorial.md`](tutorial.md); for the full
API see [`reference.md`](reference.md); for the design see
[`concepts.md`](concepts.md).

The library is `tabnas_json` (package `tabnas-json`); the engine is
`tabnas`.

## Parse a string and use the result

`parse` returns the engine's `tabnas::Value`, an enum with one variant
per JSON shape. Match on it, or index into a container:

```rust
use tabnas::Value;

let value = tabnas_json::parse(r#"{"id":7,"items":["pen","pad"],"paid":true}"#)?;
let Value::Object(order) = &value else {
    return Err("not an object".into())
};
order["id"];    // Value::Number(7.0)
order["paid"];  // Value::Bool(true)

if let Value::Array(items) = &order["items"] {
    items[1];   // Value::String("pad")
}
```

`Object` holds an `IndexMap`, so `order` iterates in document order, and
`Array` holds a `Vec`.

## Handle invalid input

Invalid JSON returns `Err(JsonError)`. Read the structured fields rather
than scraping the message:

```rust
use tabnas_json::JsonError;

fn try_parse(src: &str) -> Result<tabnas::Value, (String, usize, usize)> {
    tabnas_json::parse(src).map_err(|err: JsonError| (err.code, err.row, err.col))
}
```

The three codes this parser emits are `unexpected`,
`unterminated_string` and `invalid_unicode`. These codes are part of the
parity contract shared with the TypeScript version, so a caller matching
on them works against any of the three runtimes.

`JsonError` implements `std::error::Error` and `Display`, so it fits the
usual `?` and `Box<dyn Error>` plumbing, and printing it gives a
source-pointing message.

## Reuse a parser efficiently

The top-level `parse` already builds its engine once and reuses it, so
repeated calls do not rebuild the grammar. It is safe to call from
several threads at once, because a parse borrows the instance immutably
and builds its own context. When you need a configured parser, build one
with `make` and keep it:

```rust
let parser = tabnas_json::make();
let a = parser.parse(r#"{"x":1}"#)?;
let b = parser.parse(r#"{"y":2}"#)?;
```

To share one configured instance across threads, put it in a `OnceLock`,
which is what the crate does internally for the default:

```rust
use std::sync::OnceLock;
use tabnas::Tabnas;

static PARSER: OnceLock<Tabnas> = OnceLock::new();
let parser = PARSER.get_or_init(tabnas_json::make);
```

## Keep introspection metadata (info options)

By default the parser produces plain values. To know whether a container
was explicit, or to capture a string's quote character, enable the
engine's info options. The parsed values then come back wrapped in the
engine's carriers: `MapRef`, `ListRef` and `Text`.

```rust
use tabnas::{Options, Tabnas, Value};

let mut parser = Tabnas::new();
tabnas_json::json(&mut parser)?;
parser.set_options(|options: &mut Options| {
    options.info.map = true;
    options.info.list = true;
    options.info.text = true;
})?;

let out = parser.parse(r#"{"a":["x",1]}"#)?;

let Value::MapRef(map) = &out else { return Ok(()) };
map.implicit;                    // false: explicit braces
let Value::ListRef(list) = &map.value["a"] else { return Ok(()) };
let Value::Text(text) = &list.value[0] else { return Ok(()) };
text.quote;                      // "\""
text.string;                     // "x"
```

`MapRef.implicit` and `ListRef.implicit` record whether the container was
written explicitly, and `Text.quote` records the quote character. This is
the mode downstream plugins use when they must preserve syntax detail.

## Build a JSON-with-comments (JSONC) parser

The `json` plugin is a foundation. Install it, then re-enable comment
lexing, and the result accepts `//` and `/* */`:

```rust
use tabnas::{Options, Tabnas};

let mut jsonc = Tabnas::new();
tabnas_json::json(&mut jsonc)?;
jsonc.set_options(|options: &mut Options| {
    options.comment.lex = true;
})?;

jsonc.parse(r#"{"a":1} // trailing note"#)?; // Object {"a": 1}
jsonc.parse(r#"{"a":/* inline */2}"#)?;      // Object {"a": 2}
```

This changes only your instance. The top-level `parse` still rejects
comments.

Apply the options after `json`, never before: `json` sets
`comment.lex = false`, so an earlier call is undone by it.

## Install the grammar under your own lexer options

`json` does two things: it applies the strict JSON lexer options and it
registers the rule set, both from one grammar document. There is no
separate rules-only entry point in this port, because the options and the
`number.check` binding they name have to travel together. To extend the
grammar, install the plugin and then relax what you need:

```rust
use tabnas::{Options, Tabnas};

let mut parser = Tabnas::new();
tabnas_json::json(&mut parser)?;
parser.set_options(|options: &mut Options| {
    options.map.extend = true; // accept a trailing comma in objects
})?;
```

From here you can use the engine's rule API (`parser.rule(...)` and the
`@<rule>-<phase>` hooks) to add alternates to the shared `val`, `map`,
`list`, `pair` and `elem` rules without redeclaring the JSON core.

## Check a document against the platform parser

The crate's own test suite parses every valid fixture row with
`serde_json` as well, and the two results must agree. The same technique
is useful in an application that is migrating: parse with both, compare,
and report the input when they differ.

```rust
let mine = tabnas_json::parse(src).ok();
let theirs: Option<serde_json::Value> = serde_json::from_str(src).ok();
assert_eq!(mine.is_some(), theirs.is_some(), "disagree about {src}");
```

The two value types are different, so a full comparison walks both trees.
`rs/tests/parity_test.rs` has the version this repository uses, including
the `f64` bit comparison that keeps negative zero distinct from zero.

## Note on numbers and key ordering

Every JSON number parses to an `f64`, integers included, so `1` becomes
`Number(1.0)`. Object keys keep document order, because the engine's
`Object` variant holds an `IndexMap` rather than a `HashMap`. That
matches the TypeScript port, whose objects keep insertion order, and
differs from the Go port, where a `map[string]any` is unordered.

Two cases differ from TypeScript on purpose, both because this port
follows its own platform parser. An exponent out of `f64` range, such as
`1e999`, is syntactically valid JSON that `serde_json` rejects and
`JSON.parse` saturates to infinity, so this port rejects it. And nesting
is limited to 127 levels, which is where `serde_json` stops; past that
`parse` answers a `cancel` error. See [`concepts.md`](concepts.md) for
why parity is measured per runtime.

If you are parsing input you did not write, that second limit is the one
that matters: without it a kilobyte of open brackets would end the
process rather than return an error.

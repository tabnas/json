// Copyright (c) 2026 tabnas, MIT License

// The engine's error carries a code, position, hint and a formatted
// report, so it is large by design and `Result<_, TabnasError>` trips
// clippy's `result_large_err`. The engine allows the lint at its own
// crate root for the same reason; boxing here instead would make
// `parse` return a different shape from `Tabnas::parse` and from the
// TypeScript and Go ports, which is a worse trade than the lint.
#![allow(clippy::result_large_err)]

//! A standard JSON grammar plugin for the `tabnas` parsing engine.
//!
//! The engine ships no grammar of its own; this crate supplies the
//! strict, standard-JSON one. The rule set (`val` / `map` / `list` /
//! `pair` / `elem`) is jsonic's "Plain JSON" grammar, the pure-JSON core
//! jsonic defines before extending it for the relaxed jsonic format.
//! Here that core is installed on its own, with the lexer restricted to
//! strict JSON and none of jsonic's extended grammar (comments, unquoted
//! keys, implicit objects/arrays, trailing commas, single/backtick
//! strings, path diving).
//!
//! This plugin is intended to be the foundation other tabnas grammar
//! plugins build on: install it first, then layer additional rules on the
//! shared `val` / `map` / `list` / `pair` / `elem` rules.

use std::sync::OnceLock;

use regex::Regex;
use serde_json::json;
use tabnas::{Context, GrammarError, GrammarSpec, LexCheckResult, Tabnas, Value};

/// The README's Rust examples run as doctests, so a stale one fails the
/// gate rather than misleading the reader. Its `toml` and `bash` fences
/// are skipped; rustdoc runs only the `rust` ones.
#[cfg(doctest)]
#[doc = include_str!("../README.md")]
mod readme_examples {}

/// This crate's version. It MUST equal `ts/package.json` "version": the
/// release orchestrator rewrites both, and `tests/version_test.rs` fails
/// the build if they drift. Mirrors `VERSION` in `ts/src/json.ts` and
/// `const VERSION` in `go/json.go`.
pub const VERSION: &str = "0.5.8";

/// The error a failed parse produces, re-exported so callers need not
/// depend on the engine crate directly. Mirrors the TypeScript
/// `export { TabnasError as JsonError }`.
pub use tabnas::TabnasError as JsonError;

/// The name the serialized options bind the number preflight hook under.
/// Referencing it by name is the only way to reach `options.number.check`
/// from outside the engine crate: `LexCheck`'s constructors are
/// `pub(crate)`, and `Tabnas::lex_check_ref` exists for exactly this.
const NUMBER_CHECK: &str = "json-strict-number";

/// Exactly a standard JSON number.
fn strict_number() -> &'static Regex {
    static RE: OnceLock<Regex> = OnceLock::new();
    RE.get_or_init(|| {
        Regex::new(r"^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$")
            .expect("the strict-number pattern is a literal and compiles")
    })
}

/// The candidate literal starting at `src`, up to the next JSON
/// structural character or whitespace. That boundary is what the engine's
/// (lenient) number matcher would consider, so it is what has to be
/// judged.
///
/// `/` is a boundary too, and not for strict JSON's sake: a slash after a
/// number is invalid there whatever this returns. It is for the JSONC
/// recipe the docs describe. With comment lexing re-enabled,
/// `{"a":1/* note */}` has no space between the number and the comment,
/// so without `/` here the literal scanned to the next WHITESPACE and the
/// hook judged `1/*`, failed the pattern, and answered `Skip` -- turning
/// a valid JSONC document into `unexpected`. Stopping here lets the
/// number tokenize and leaves the comment to the lexer, which is the one
/// that knows whether comments are on.
fn leading_literal(src: &str) -> &str {
    let end = src
        .find(|c: char| c.is_whitespace() || matches!(c, ',' | '}' | ']' | ':' | '"' | '/'))
        .unwrap_or(src.len());
    &src[..end]
}

/// Reject anything the engine's lenient number matcher would accept that
/// standard JSON does not.
///
/// This is a `check` hook rather than `options.number.exclude` for two
/// independent reasons, and both matter:
///
/// 1. **`exclude` cannot express it.** The TypeScript exclude is a
///    negative lookahead (`/^(?!-?(?:0|[1-9][0-9]*)…$)/`), and the Rust
///    engine's `exclude` is a pattern for the `regex` crate, which has no
///    lookaround at all. The positive form plus an inversion is the only
///    way to say it here, which is the shape the Go port already uses.
///
/// 2. **Out-of-range exponents.** `1e999` and `123123e100000` are
///    syntactically valid JSON, and the platform oracles disagree about
///    them: `JSON.parse` saturates to `Infinity` (so TypeScript accepts),
///    while `encoding/json` errors. `serde_json` — this runtime's oracle —
///    errors too ("number out of range"), verified, so Rust rejects them
///    with Go rather than accepting with TypeScript. AGENTS.md rule 4 is
///    per-runtime parity and names this as a deliberate, permanent
///    asymmetry. Underflow (`1e-999` -> `0`) is accepted by serde_json and
///    is left alone, exactly as Go leaves it.
///
/// Note the asymmetry is not expressible as a regex either way, which is
/// the deeper reason both ports need a predicate and TypeScript does not.
fn strict_number_check(src: &str) -> LexCheckResult {
    let literal = leading_literal(src);

    // The hook runs wherever a number COULD be lexed, not only where one
    // starts, so say nothing unless a number-ish literal is actually here.
    let starts_number = literal
        .chars()
        .next()
        .is_some_and(|c| c == '-' || c == '+' || c == '.' || c.is_ascii_digit());
    if !starts_number {
        return LexCheckResult::Continue;
    }

    if !strict_number().is_match(literal) {
        return LexCheckResult::Skip;
    }

    // Syntactically standard, but out of f64 range. `parse` saturates to
    // an infinity rather than failing, so the finiteness test is the check.
    match literal.parse::<f64>() {
        Ok(value) if !value.is_finite() => LexCheckResult::Skip,
        Ok(_) => LexCheckResult::Continue,
        Err(_) => LexCheckResult::Skip,
    }
}

/// serde_json's own nesting limit, and therefore this port's.
///
/// `serde_json::from_str` accepts 127 levels of nesting and refuses the
/// 128th with "recursion limit exceeded"; `JSON.parse` and
/// `encoding/json` both go far deeper. That is the same shape of
/// platform disagreement as the out-of-range exponent above, and
/// per-runtime parity answers it the same way: this port follows its own
/// platform. The boundary was measured against serde_json rather than
/// read off its constant, and `tests/json_test.rs` re-measures it, so a
/// future change there shows up as a failure instead of as silent drift.
///
/// Unlike that one, it is also a crash fix. Without a limit, a 1 KB
/// source of 500 open brackets aborts the process with a stack overflow
/// rather than returning an error, which the external conformance corpus
/// exercises directly (`i_structure_500_nested_arrays`,
/// `n_structure_100000_opening_arrays`). A parser reached with untrusted
/// input must not be able to end the process.
const DEPTH_LIMIT: usize = 127;

/// How many containers are open at this point in the parse.
///
/// Unlike the number check, the budget needs no name: `parse_budget`
/// takes the closure directly, so there is nothing to bind by name and
/// nothing for the grammar document to reference.
///
/// Counted from the RULE NAMES rather than from `rule_stack.len()`. The
/// stack holds about three rules per level (`val`, then `map`/`list`,
/// then `pair`/`elem`), so a length-based limit would encode that ratio
/// and shift silently the first time the grammar gains an alternate.
/// Counting the container rules is the depth a reader of the document
/// would count.
///
/// The rule the loop is working on is NOT in `rule_stack`: the engine
/// hands it over separately as `context.rule`, and the stack holds only
/// its ancestors. A container is open from the moment it is that rule,
/// so it has to be counted too. Counting the ancestors alone made the
/// boundary depend on what the innermost container held: `[]` nested 127
/// deep parsed, because the 127th list was the current rule and went
/// uncounted, while `[1]` nested 127 deep was refused, because by the
/// time `1` was read all 127 lists were ancestors. serde_json accepts
/// both, and `tests/json_test.rs` now measures both shapes against it.
fn depth(context: &Context) -> usize {
    let is_container = |name: &str| name == "map" || name == "list";
    let ancestors = context
        .rule_stack
        .iter()
        .filter(|rule| is_container(&rule.name))
        .count();
    let current = usize::from(
        context
            .rule
            .as_ref()
            .is_some_and(|rule| is_container(&rule.name)),
    );
    ancestors + current
}

/// The parse budget: stop before the nesting outruns the stack.
///
/// At most, not strictly less than. `depth` already includes the
/// container the loop is inside, so the count it returns IS the nesting
/// depth of the token about to be read, and `DEPTH_LIMIT` means "this
/// many levels parse, the next one does not": the 128th container fails
/// the check on the very iteration it becomes the current rule, whether
/// it turns out to be empty or not. That is the boundary
/// `tests/json_test.rs` measures against serde_json rather than
/// asserting from this reasoning.
fn within_depth_limit(context: &Context) -> bool {
    depth(context) <= DEPTH_LIMIT
}

/// The one serialized document carrying both the strict-JSON options and
/// the JSON rule set, mirroring `JSON_OPTIONS` + `registerJsonGrammar` in
/// `ts/src/json.ts` and `jsonOptions` + `RegisterJSONGrammar` in
/// `go/json.go`.
///
/// Options travel in the grammar document rather than through the typed
/// `Options` struct because `number.check` can only be bound by name from
/// here; everything else could go either way, and keeping them together
/// means there is one definition of "strict JSON" rather than two halves
/// that can drift.
fn json_document() -> serde_json::Value {
    json!({
        // The schema version of the native-value builtins this grammar
        // binds to (object/array/reset/key/setval/push/value).
        "v": 2,

        "options": {
            "text": { "lex": false },
            "number": {
                "hex": false, "oct": false, "bin": false, "sep": null,
                "check": NUMBER_CHECK,
            },
            "string": {
                "chars": "\"",
                "multiChars": "",
                // Standard JSON escape handling: allowUnknown:false
                // rejects any unrecognized escape (\q, \z); escapeStrict
                // disables the engine's non-standard \xHH and \u{...}
                // structural escapes (plain \uXXXX stays); and dropping
                // v / ' / ` from the escape map removes the remaining
                // non-standard built-ins. Result: exactly the JSON escape
                // set, identical to the other two runtimes.
                //
                // NOTE the entries are `null`, where ts/src/json.ts writes
                // `''`. Same intent, different idiom: this engine deletes
                // an escape entry on a null value and treats `""` as a
                // real mapping TO the empty string, so `""` here would
                // make `"\v"` parse as `""` instead of being rejected --
                // which is exactly what it did before this comment
                // existed, on five spec rows.
                "allowUnknown": false,
                "escapeStrict": true,
                "escape": { "v": null, "'": null, "`": null },
            },
            "comment": { "lex": false },
            "map": { "extend": false },
            "lex": { "empty": false },
            // Restrict the rule set to the `json`-tagged alternates. The
            // grammar below tags every alt "json", so on a bare engine
            // this is inert; it matters when these options are applied
            // over an already-extended grammar, keeping only its
            // strict-JSON alternates.
            "rule": { "finish": false, "include": "json" },
            // Strict JSON keys are quoted strings only.
            "tokenSet": { "KEY": ["#ST"] },
        },

        // The value tree is built ENTIRELY by the engine's native-value
        // `$`-builtins, referenced by name on the alts below; the engine
        // merges them in at load. There are NO grammar-local closures.
        //
        //   @reset$  - clear the parent-seeded node.
        //   @object$ - allocate an empty object into the node.
        //   @array$  - allocate an empty array into the node.
        //   @key$    - capture the matched key for the pending @setval$.
        //   @setval$ - assign the built child value under that key.
        //   @push$   - append the built child value to the array.
        //   @value$  - resolve the rule's value (child wins, else token).
        "rule": {
            "val": {
                "open": [
                    { "s": "#OB", "p": "map",  "b": 1, "a": "@reset$", "g": "map,json" },
                    { "s": "#OS", "p": "list", "b": 1, "a": "@reset$", "g": "list,json" },
                    { "s": "#VAL", "a": "@reset$", "g": "val,json" },
                ],
                "close": [
                    { "s": "#ZZ", "a": "@value$", "g": "end,json" },
                    { "b": 1, "a": "@value$", "g": "more,json" },
                ],
            },
            "map": {
                "open": [
                    { "s": "#OB #CB", "b": 1, "n": { "pk": 0 }, "a": "@object$", "g": "map,json" },
                    { "s": "#OB", "p": "pair", "n": { "pk": 0 }, "a": "@object$", "g": "map,json,pair" },
                ],
                "close": [ { "s": "#CB", "g": "end,json" } ],
            },
            "list": {
                "open": [
                    { "s": "#OS #CS", "b": 1, "a": "@array$", "g": "list,json" },
                    { "s": "#OS", "p": "elem", "a": "@array$", "g": "list,elem,json" },
                ],
                "close": [ { "s": "#CS", "g": "end,json" } ],
            },
            "pair": {
                "open": [
                    { "s": "#KEY #CL", "p": "val", "u": { "pair": true },
                      "a": "@key$", "g": "map,pair,key,json" },
                ],
                "close": [
                    { "s": "#CA", "r": "pair", "a": "@setval$", "g": "map,pair,comma,json" },
                    { "s": "#CB", "b": 1, "a": "@setval$", "g": "map,pair,close,json" },
                ],
            },
            // `"r": "elem"` REPLACES this rule, and `push$.chain: false`
            // says no rule in the assembled grammar will resolve `$prev`
            // to read the rule it displaced. It is unconditional here,
            // where the TypeScript and Go ports make it an opt-in their
            // rules-only installers leave off: this port has no such
            // installer (see `json`'s docs), so this grammar IS the
            // assembled grammar and the claim is the port's to make.
            //
            // It buys nothing either way. A list here is one shared
            // array that every view already sees grow; the walk it skips
            // is O(elements^2) only in Go, where a list is a slice VALUE.
            // The key is declared so the three grammars stay one grammar.
            "elem": {
                "open": [ { "p": "val", "g": "list,elem,val,json" } ],
                "close": [
                    { "s": "#CA", "r": "elem", "a": "@push$",
                      "k": { "push$": { "chain": false } },
                      "g": "list,elem,comma,json" },
                    { "s": "#CS", "b": 1, "a": "@push$",
                      "k": { "push$": { "chain": false } },
                      "g": "list,elem,close,json" },
                ],
            },
        },

        // Declared order, matching the order ts/src/json.ts declares the
        // rules in. Without it the engine falls back to sorted names and
        // anything reading rule order (railroad's extracted model among
        // them) would report this grammar alphabetically.
        "ruleOrder": ["val", "map", "list", "pair", "elem"],
    })
}

/// Install the strict JSON options and the JSON rule set on `parser`.
///
/// This is the one entry point: `make` goes through it too, so the two
/// construction paths cannot drift apart.
///
/// ```
/// let mut parser = tabnas::Tabnas::new();
/// tabnas_json::json(&mut parser)?;
/// let value = parser.parse("[1,2,3]")?;
/// assert_eq!(value.to_string(), "[1,2,3]");
/// # Ok::<(), Box<dyn std::error::Error>>(())
/// ```
pub fn json(parser: &mut Tabnas) -> Result<(), GrammarError> {
    parser.lex_check_ref(NUMBER_CHECK, strict_number_check);
    let spec = GrammarSpec::from_value(json_document())?;
    parser.grammar(&spec)?;
    // AFTER the grammar, not before: `grammar` applies the document's
    // options, and an options pass that does not mention `parse.budget`
    // is not required to preserve one set earlier. Setting it here is
    // also the same ordering rule `make` documents for caller options.
    //
    // Every iteration, because the check is what stands between a deeply
    // nested source and a stack overflow; a sampled check would let the
    // parse run past the limit by however many levels the sample missed.
    parser.parse_budget(1, within_depth_limit);
    Ok(())
}

/// Build a standard-JSON parser instance.
///
/// Infallible by design, and it goes through [`json()`] rather than
/// duplicating the setup, so this path and installing the plugin by hand
/// cannot drift. The document is a fixed literal, so a failure here is a
/// bug in this crate rather than anything a caller did — the Go `Make`
/// panics for the same reason, with the same justification.
///
/// ```
/// let parser = tabnas_json::make();
/// let value = parser.parse("[1,2,3]")?;
/// assert_eq!(value.to_string(), "[1,2,3]");
/// assert!(parser.parse("[1,2,]").is_err());
/// # Ok::<(), tabnas_json::JsonError>(())
/// ```
pub fn make() -> Tabnas {
    let mut parser = Tabnas::new();
    json(&mut parser).expect("the JSON grammar document is fixed and valid");
    parser
}

/// Parse a JSON source string with the shared default parser.
///
/// The engine is built once, on first use, and reused after that. Both
/// other runtimes do the same (`sync.Once` in `go/json.go`, a lazily
/// assigned module variable in `ts/src/json.ts`), and reuse is safe here
/// for the same reason it is there: [`Tabnas::parse`] takes `&self` and
/// builds a fresh parse context per call, and `Tabnas` is `Send + Sync`,
/// so concurrent callers share one installed grammar instead of each
/// rebuilding it. `tests/json_test.rs` pins that with a threaded test.
///
/// Use [`make`] instead when the parser needs configuring: that returns a
/// fresh instance and leaves this one alone.
///
/// ```
/// let value = tabnas_json::parse(r#"{"a":[1,2]}"#)?;
/// assert_eq!(value.to_string(), r#"{"a":[1,2]}"#);
/// assert_eq!(tabnas_json::parse("{a:1}").unwrap_err().code, "unexpected");
/// # Ok::<(), tabnas_json::JsonError>(())
/// ```
pub fn parse(src: &str) -> Result<Value, JsonError> {
    static DEFAULT: OnceLock<Tabnas> = OnceLock::new();
    DEFAULT.get_or_init(make).parse(src)
}

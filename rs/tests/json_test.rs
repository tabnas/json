// In-language tests: the behaviour the shared fixtures do not pin, and the
// deliberate runtime asymmetries this port carries.

use tabnas::Value;
use tabnas_json::{make, parse};

#[test]
fn parses_the_json_value_shapes() {
    assert!(matches!(
        parse("{}").expect("empty object"),
        Value::Object(_)
    ));
    assert!(matches!(parse("[]").expect("empty array"), Value::Array(_)));
    assert_eq!(parse("null").expect("null").to_string(), "null");
    assert_eq!(parse("true").expect("true").to_string(), "true");
}

#[test]
fn rejects_the_extended_grammar() {
    // None of jsonic's relaxations are in the strict grammar.
    for src in [
        "{a:1}",      // unquoted key
        "[1,2,]",     // trailing comma
        "1 // note",  // comment
        "'x'",        // single-quoted string
        "{\"a\":1,}", // trailing comma in a map
    ] {
        assert!(parse(src).is_err(), "should have rejected: {src}");
    }
}

#[test]
fn rejects_non_standard_numbers() {
    // The four shapes the engine's lenient number matcher accepts and
    // standard JSON does not.
    for src in ["+1", ".5", "1.", "01", "00", "-01"] {
        assert!(parse(src).is_err(), "should have rejected: {src}");
    }
    // ... while the standard ones still parse.
    for src in ["0", "-0", "1", "-1", "1.5", "1e3", "1E3", "1e+3", "1e-3"] {
        assert!(parse(src).is_ok(), "should have accepted: {src}");
    }
}

// This is the port's one deliberate divergence from the canonical
// TypeScript, and it is the Go behaviour rather than the TS behaviour.
//
// `1e999` is syntactically valid JSON, and the platform oracles disagree:
// `JSON.parse` saturates to Infinity (TypeScript accepts), while
// `encoding/json` errors. serde_json — this runtime's oracle — errors too,
// so Rust rejects with Go. AGENTS.md rule 4 is per-runtime parity and
// names this as a permanent asymmetry, so pin it here the way
// `TestNumberOverflowRejected` pins the Go half.
#[test]
fn rejects_out_of_range_exponents_like_its_platform_oracle() {
    for src in ["1e999", "1e309", "123123e100000", "-1e999"] {
        assert!(parse(src).is_err(), "should have rejected overflow: {src}");
        // ... and the oracle agrees, which is WHY this port rejects it.
        assert!(
            serde_json::from_str::<serde_json::Value>(src).is_err(),
            "serde_json should reject it too, or this divergence is wrong: {src}"
        );
    }

    // Underflow is accepted by serde_json, so it is accepted here too --
    // Go leaves it alone for the same reason.
    assert!(parse("1e-999").is_ok());
    assert!(serde_json::from_str::<serde_json::Value>("1e-999").is_ok());
}

#[test]
fn rejects_the_non_standard_escapes() {
    // Dropped from the escape map, so these are unknown escapes and
    // allowUnknown:false rejects them. They parsed as an empty string
    // until the map entries became `null` rather than `""`.
    for src in [r#""\v""#, r#""\'""#, r#""\`""#] {
        assert!(parse(src).is_err(), "should have rejected escape: {src}");
    }
    // The standard set still works.
    assert!(parse(r#""\n\t\r\\\/\"\b\fA""#).is_ok());
}

#[test]
fn empty_input_is_not_a_value() {
    assert!(parse("").is_err());
    assert!(parse("   ").is_err());
}

#[test]
fn make_and_parse_agree() {
    // `parse` goes through `make`, which goes through the plugin, so the
    // two construction paths cannot drift.
    let value = make().parse(r#"{"a":[1,2]}"#).expect("parses");
    let direct = parse(r#"{"a":[1,2]}"#).expect("parses");
    assert_eq!(value.to_string(), direct.to_string());
}

#[test]
fn an_instance_is_reusable() {
    let parser = make();
    assert!(parser.parse("1").is_ok());
    assert!(parser.parse("[2]").is_ok());
    assert!(parser.parse("{bad").is_err());
    // Still usable after a failure.
    assert!(parser.parse("3").is_ok());
}

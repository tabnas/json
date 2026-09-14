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

#[test]
fn the_shared_default_parser_takes_concurrent_callers() {
    // `parse` builds its engine once and hands every caller the same one,
    // as `sync.Once` does in the Go port. Sharing is pinned by the
    // compiler (a `&mut self` parse would not fit in a `OnceLock`); what
    // is NOT pinned by the compiler, and is what this holds, is that the
    // shared engine keeps no state between parses. Failing parses are
    // interleaved with succeeding ones on purpose: a lexer or rule-stack
    // leak across calls would surface as a wrong value or a spurious
    // error here, and nowhere else in a suite that is otherwise
    // single-threaded and one-parse-per-instance.
    let threads: Vec<_> = (0..8)
        .map(|n| {
            std::thread::spawn(move || {
                let src = format!(r#"{{"n":{n},"xs":[1,2,3]}}"#);
                for _ in 0..50 {
                    let value = parse(&src).expect("parses");
                    let Value::Object(fields) = &value else {
                        panic!("an object, got {value:?}")
                    };
                    assert_eq!(fields["n"], Value::Number(f64::from(n)));
                    assert!(parse("{bad").is_err());
                }
            })
        })
        .collect();
    for thread in threads {
        thread.join().expect("no thread panicked");
    }
}

#[test]
fn rejects_nesting_deeper_than_its_platform_oracle() {
    // serde_json accepts 127 levels and refuses the 128th; `JSON.parse`
    // and `encoding/json` both go far deeper. Per-runtime parity answers that the same way it
    // answers the out-of-range exponent above: follow this platform.
    //
    // Asserted against the oracle, not against a remembered number, so the
    // day serde_json moves its limit this test says so instead of quietly
    // encoding the old one.
    let nest = |n: usize| format!("{}{}", "[".repeat(n), "]".repeat(n));

    for depth in [1usize, 2, 64, 127] {
        let src = nest(depth);
        assert!(parse(&src).is_ok(), "depth {depth} must parse");
        assert!(
            serde_json::from_str::<serde_json::Value>(&src).is_ok(),
            "depth {depth}: serde_json must accept it too"
        );
    }

    for depth in [128usize, 129, 500, 5_000] {
        let src = nest(depth);
        assert!(parse(&src).is_err(), "depth {depth} must be rejected");
        assert!(
            serde_json::from_str::<serde_json::Value>(&src).is_err(),
            "depth {depth}: serde_json must reject it too"
        );
    }

    // The point of the limit. Without it this input aborts the process
    // with a stack overflow rather than returning an error, which no
    // amount of caller care can defend against.
    assert!(
        parse(&nest(100_000)).is_err(),
        "a deep source must not crash"
    );
}

#[test]
fn the_jsonc_recipe_takes_a_comment_against_a_number() {
    // The layering recipe the docs describe, with no space between the
    // number and the comment. That is the case the number preflight hook
    // got wrong: it scanned the candidate literal to the next WHITESPACE,
    // so it judged `1/*` rather than `1`, failed the strict pattern and
    // answered Skip -- turning valid JSONC into `unexpected`. The spaced
    // forms worked, which is why nothing caught it.
    let mut jsonc = tabnas::Tabnas::new();
    tabnas_json::json(&mut jsonc).expect("plugin installs");
    jsonc
        .set_options(|options: &mut tabnas::Options| {
            options.comment.lex = true;
        })
        .expect("options apply");

    for src in [
        r#"{"a":1/* note */}"#,
        "{\"a\":1//note\n}",
        r#"[1/* x */,2]"#,
        r#"{"a":1 /* spaced */}"#,
    ] {
        assert!(jsonc.parse(src).is_ok(), "JSONC must accept {src:?}");
    }

    // And none of that loosens strict JSON, which has no comments at all.
    for src in [r#"{"a":1/* note */}"#, "1/2", "{\"a\":1//x\n}"] {
        assert!(parse(src).is_err(), "strict JSON must reject {src:?}");
    }
}

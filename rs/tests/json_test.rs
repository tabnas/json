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
fn the_depth_boundary_does_not_depend_on_what_the_innermost_container_holds() {
    // The test above nests EMPTY arrays, and that shape alone agreed with
    // serde_json while every other one was off by a level: `[]` nested
    // 127 deep parsed but `[1]` nested 127 deep answered `cancel`, and so
    // did 127 nested objects, because the engine hands the budget check
    // the rule it is inside separately from the ancestor stack, and only
    // the ancestors were counted. An empty 127th container was the current
    // rule, and uncounted; a scalar inside it saw all 127 as ancestors.
    //
    // serde_json's limit is a count of open containers, whatever they
    // hold, so the boundary here has to be the same for every shape. Each
    // shape is asserted against the oracle, not against the number.
    fn wrap(open: &str, n: usize, inner: &str, close: &str) -> String {
        format!("{}{inner}{}", open.repeat(n), close.repeat(n))
    }
    type Shape = fn(usize) -> String;
    let shapes: [(&str, Shape); 7] = [
        ("arrays around a scalar", |n| wrap("[", n, "1", "]")),
        ("arrays around a string", |n| wrap("[", n, "\"x\"", "]")),
        ("arrays around two elements", |n| wrap("[", n, "1,2", "]")),
        ("objects around a scalar", |n| wrap("{\"a\":", n, "1", "}")),
        // n containers in total: n - 1 objects plus the empty one inside.
        ("objects around an empty object", |n| {
            wrap("{\"a\":", n - 1, "{}", "}")
        }),
        ("arrays around an empty object", |n| {
            wrap("[", n - 1, "{}", "]")
        }),
        // A sibling after the deep branch: the depth has to unwind.
        ("a deep branch then a sibling", |n| {
            format!("{{\"a\":{},\"b\":1}}", wrap("[", n - 1, "1", "]"))
        }),
    ];

    for (name, shape) in &shapes {
        let ok = shape(127);
        assert!(
            parse(&ok).is_ok(),
            "{name}: 127 levels must parse: {:?}",
            parse(&ok).err().map(|e| e.code)
        );
        assert!(
            serde_json::from_str::<serde_json::Value>(&ok).is_ok(),
            "{name}: serde_json must accept 127 levels too"
        );

        let deep = shape(128);
        let error = parse(&deep).expect_err(&format!("{name}: 128 levels must be rejected"));
        assert_eq!(
            error.code, "cancel",
            "{name}: the rejection is the budget's"
        );
        assert!(
            serde_json::from_str::<serde_json::Value>(&deep).is_err(),
            "{name}: serde_json must reject 128 levels too"
        );
    }
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

#[test]
fn integer_like_keys_keep_document_order_like_its_platform_oracle() {
    // A JavaScript object enumerates integer-like keys FIRST, in ascending
    // numeric order, whatever order the document wrote them: the
    // TypeScript port reads `{"2":"a","1":"b"}` back as 1, 2. An IndexMap
    // does not do that, and neither does serde_json, so this port keeps
    // document order.
    //
    // Per-runtime parity again, not a defect. Pinned here because no
    // shared fixture has an integer-like object key -- which is exactly
    // why the docs claimed key order matched TypeScript until a review
    // said otherwise.
    let src = r#"{"2":"a","1":"b","x":"c"}"#;

    let Value::Object(ours) = parse(src).expect("parses") else {
        panic!("an object")
    };
    let keys: Vec<&str> = ours.keys().map(String::as_str).collect();
    assert_eq!(keys, ["2", "1", "x"], "document order, not numeric order");

    // And the oracle agrees, which is the half that makes this a parity
    // claim rather than a preference.
    let oracle: serde_json::Value = serde_json::from_str(src).expect("parses");
    let serde_json::Value::Object(theirs) = oracle else {
        panic!("an object")
    };
    let oracle_keys: Vec<&str> = theirs.keys().map(String::as_str).collect();
    assert_eq!(keys, oracle_keys, "serde_json orders them the same way");
}

#[test]
fn a_raw_control_character_in_a_string_is_unprintable() {
    // The engine answers `unprintable` for a raw character below U+0020
    // inside a string, in all three runtimes (the TypeScript and Go
    // lexers emit the same code), yet no shared fixture row pins the
    // code and the reference table did not list it. serde_json rejects
    // every one of these too, which is what makes it a parity claim
    // rather than a preference.
    for src in ["\"a\u{1}b\"", "\"a\tb\"", "\"a\nb\"", "\"\u{0}\""] {
        let error = parse(src).expect_err("a raw control character is not JSON");
        assert_eq!(error.code, "unprintable", "{src:?}");
        assert!(
            serde_json::from_str::<serde_json::Value>(src).is_err(),
            "serde_json must reject it too: {src:?}"
        );
    }
    // The escaped forms are the JSON way to write the same characters.
    assert_eq!(
        parse(r#""a\tb\n""#).expect("escapes parse"),
        Value::String("a\tb\n".into())
    );
}

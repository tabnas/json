// The shared conformance fixtures, plus this runtime's platform oracle.
//
// The oracle half is the point, and it mirrors what the Go runner does
// with encoding/json: a `valid` row is not merely compared to the
// fixture's expected value, it is ALSO parsed by serde_json and the two
// results must agree. AGENTS.md rule 4 holds each runtime to its own
// platform JSON parser, so the check belongs in the suite rather than in
// a comment claiming it.

mod common;

use common::oracle::{same, to_json};
use common::spec;

/// The comparator is a claim about what it REJECTS, and a clean run over
/// agreeing values cannot tell a working one from `_ , _ => true`. Signed
/// zero is the case it exists for, so it is the case pinned here.
#[test]
fn the_comparator_keeps_signed_zero_apart() {
    let neg: serde_json::Value = serde_json::from_str("-0").expect("parses");
    let pos: serde_json::Value = serde_json::from_str("0").expect("parses");
    assert!(!same(&neg, &pos), "-0 and 0 must not compare equal");
    assert!(same(&neg, &neg) && same(&pos, &pos), "each equals itself");

    // Whole numbers still compare across serde_json's lexical forms, which
    // is what the numeric comparison is for.
    let int: serde_json::Value = serde_json::from_str("1").expect("parses");
    let float: serde_json::Value = serde_json::from_str("1.0").expect("parses");
    assert!(same(&int, &float), "1 and 1.0 are the same number");
}

#[test]
fn spec() {
    let mut failures = Vec::new();

    for file in spec::files() {
        for row in spec::rows(&file) {
            let at = format!("{}:{}", row.file, row.line);
            let result = tabnas_json::parse(&row.input);

            if let Some(code) = row.expected.strip_prefix("ERROR:") {
                match result {
                    Ok(value) => failures.push(format!(
                        "{at}: expected error {code}, parsed {:?}",
                        to_json(&value)
                    )),
                    // The CODE field, compared for equality. This searched
                    // the rendered diagnostic for the code as a SUBSTRING,
                    // which is not the contract and is not even a reliable
                    // proxy for it: `unexpected` is a substring of a
                    // hypothetical `unexpected_eof`, and the report quotes
                    // the offending source, so a row whose own input held
                    // the word would have passed whatever the parser did.
                    // AGENTS.md rule 3 makes the code itself the shared
                    // contract, so assert the code itself.
                    Err(error) if error.code != code => {
                        failures.push(format!("{at}: expected error {code}, got {}", error.code))
                    }
                    Err(_) => {}
                }
                continue;
            }

            let value = match result {
                Ok(value) => value,
                Err(error) => {
                    failures.push(format!(
                        "{at}: expected {}, got error {}",
                        row.expected,
                        error.to_string().lines().next().unwrap_or_default()
                    ));
                    continue;
                }
            };

            let got = to_json(&value);

            // 1. The fixture's expected value, shared with the other runtimes.
            match serde_json::from_str::<serde_json::Value>(&row.expected) {
                Ok(want) if same(&got, &want) => {}
                Ok(want) => {
                    failures.push(format!("{at}: got {got}, fixture expects {want}"));
                    continue;
                }
                Err(e) => {
                    failures.push(format!("{at}: fixture expected value is not JSON: {e}"));
                    continue;
                }
            }

            // 2. This runtime's oracle. A fixture the platform parser
            //    rejects, or reads differently, is a bug in this port.
            match serde_json::from_str::<serde_json::Value>(&row.input) {
                Ok(oracle) if same(&got, &oracle) => {}
                Ok(oracle) => failures.push(format!(
                    "{at}: disagrees with serde_json: {got} != {oracle}"
                )),
                Err(e) => failures.push(format!("{at}: serde_json rejected a valid fixture: {e}")),
            }
        }
    }

    spec::report(failures);
}

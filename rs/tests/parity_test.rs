// The shared conformance fixtures, plus this runtime's platform oracle.
//
// The oracle half is the point, and it mirrors what the Go runner does
// with encoding/json: a `valid` row is not merely compared to the
// fixture's expected value, it is ALSO parsed by serde_json and the two
// results must agree. AGENTS.md rule 4 holds each runtime to its own
// platform JSON parser, so the check belongs in the suite rather than in
// a comment claiming it.

mod common;

use common::spec;
use tabnas::Value;

/// The engine's value as serde_json's, so the two can be compared.
fn to_json(value: &Value) -> serde_json::Value {
    match value {
        Value::Null => serde_json::Value::Null,
        Value::Bool(b) => serde_json::Value::Bool(*b),
        Value::Number(n) => serde_json::Number::from_f64(*n)
            .map(serde_json::Value::Number)
            .unwrap_or(serde_json::Value::Null),
        Value::String(s) => serde_json::Value::String(s.clone()),
        Value::Array(items) => serde_json::Value::Array(items.iter().map(to_json).collect()),
        Value::Object(entries) => serde_json::Value::Object(
            entries
                .iter()
                .map(|(k, v)| (k.clone(), to_json(v)))
                .collect(),
        ),
        other => serde_json::Value::String(format!("<unrepresentable: {other:?}>")),
    }
}

/// Deep equality that compares numbers numerically.
///
/// serde_json keeps the lexical form (`0` is an integer `Number`, `0.0` a
/// float), while every value this engine produces is an f64. Comparing
/// the reprs would fail every whole-number row for a difference that is
/// not one. Go's runner normalises the same way.
fn same(a: &serde_json::Value, b: &serde_json::Value) -> bool {
    use serde_json::Value as J;
    match (a, b) {
        (J::Number(x), J::Number(y)) => match (x.as_f64(), y.as_f64()) {
            // `to_bits` rather than `==` so -0.0 and 0.0 stay distinct,
            // which the value contract keeps and the fixtures rely on.
            (Some(x), Some(y)) => x.to_bits() == y.to_bits() || x == y,
            _ => x == y,
        },
        (J::Array(x), J::Array(y)) => {
            x.len() == y.len() && x.iter().zip(y).all(|(x, y)| same(x, y))
        }
        (J::Object(x), J::Object(y)) => {
            // Zipped rather than keyed, so KEY ORDER is compared too --
            // the TypeScript runner pins it by comparing renderings, Go in
            // a separate order test; doing it here keeps it in one place.
            x.len() == y.len()
                && x.iter()
                    .zip(y)
                    .all(|((xk, xv), (yk, yv))| xk == yk && same(xv, yv))
        }
        _ => a == b,
    }
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
                    Err(error) => {
                        let text = error.to_string();
                        if !text.contains(code) {
                            failures.push(format!(
                                "{at}: expected error {code}, got {}",
                                text.lines().next().unwrap_or_default()
                            ));
                        }
                    }
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

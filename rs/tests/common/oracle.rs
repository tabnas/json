// Comparing an engine value with a serde_json one.
//
// Shared by `parity_test.rs` (the fixture rows) and `conformance_test.rs`
// (the external corpus) so the two cannot come to different conclusions
// about what "the same value" means.

use tabnas::Value;

/// The engine's value as serde_json's, so the two can be compared.
pub fn to_json(value: &Value) -> serde_json::Value {
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
pub fn same(a: &serde_json::Value, b: &serde_json::Value) -> bool {
    use serde_json::Value as J;
    match (a, b) {
        (J::Number(x), J::Number(y)) => match (x.as_f64(), y.as_f64()) {
            // `to_bits`, NOT `==`, so -0.0 and 0.0 stay distinct: the
            // value contract keeps the sign and `valid.tsv` has two rows
            // (`-0` and `-0.0`) that assert it.
            //
            // This read `to_bits() == to_bits() || x == y`, which was the
            // same thing as plain `==`: the two comparisons differ ONLY on
            // signed zero and NaN, so the `||` arm re-admitted exactly the
            // case the `to_bits` arm existed to reject. Both zeros are
            // finite and equal under IEEE `==`, so `same(0.0, -0.0)` was
            // true and the comment above it was false.
            //
            // Dropping the arm loses nothing: for every other pair of
            // finite f64 values, equal bits and `==` agree, and NaN is
            // rejected by both.
            (Some(x), Some(y)) => x.to_bits() == y.to_bits(),
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

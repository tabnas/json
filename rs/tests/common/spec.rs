// Loader for the shared `test/spec/*.tsv` conformance fixtures.
//
// `@tabnas/support` has no Rust half, so this is the third independent
// implementation of the one format, beside the TypeScript runner and the
// Go one. It is therefore the loader that can DRIFT: keep it to the
// format `../../test/AGENTS.md` pins -- a header row naming the columns,
// tab-separated, blank lines skipped, and a comment line being one that
// starts with `#` and contains no tab.

use std::fs;
use std::path::{Path, PathBuf};

pub struct Row {
    pub file: String,
    pub line: usize,
    pub input: String,
    pub expected: String,
}

/// Undo the escapes the TSV format uses for characters that cannot appear
/// raw in a tab-separated line.
fn unescape(text: &str) -> String {
    let mut out = String::with_capacity(text.len());
    let mut chars = text.chars();
    while let Some(c) = chars.next() {
        if c != '\\' {
            out.push(c);
            continue;
        }
        match chars.next() {
            Some('n') => out.push('\n'),
            Some('r') => out.push('\r'),
            Some('t') => out.push('\t'),
            Some('\\') => out.push('\\'),
            Some(other) => {
                out.push('\\');
                out.push(other);
            }
            None => out.push('\\'),
        }
    }
    out
}

pub fn spec_dir() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("rs/ has a parent")
        .join("test")
        .join("spec")
}

/// Every data row of one fixture file, in file order.
pub fn rows(file: &str) -> Vec<Row> {
    let path = spec_dir().join(file);
    let body =
        fs::read_to_string(&path).unwrap_or_else(|e| panic!("cannot read {}: {e}", path.display()));

    let mut out = Vec::new();
    for (index, raw) in body.lines().enumerate() {
        let line = index + 1;
        if raw.trim().is_empty() {
            continue;
        }
        // A comment is a `#` line with no tab; a data row always has one.
        if raw.starts_with('#') && !raw.contains('\t') {
            continue;
        }
        let mut cols = raw.split('\t');
        let input = cols.next().unwrap_or_default();
        let expected = cols.next().unwrap_or_default();
        // The header row names the columns.
        if line == 1 && input == "input" {
            continue;
        }
        out.push(Row {
            file: file.to_string(),
            line,
            // The INPUT is escape-decoded; the EXPECTED column is not.
            // That asymmetry is the canonical runner's (`unesc(0)` for the
            // source, `parseExpected: (expected) => expected` for the
            // value), and it is load-bearing: a fixture cell like
            // `"a\\b"` means the two-character JSON escape, which the
            // expected side must keep so its own parser sees the same
            // source the engine did. Decoding both collapses one layer
            // twice and quietly changes what the row asserts.
            input: unescape(input),
            expected: expected.to_string(),
        });
    }
    assert!(!out.is_empty(), "{} has no data rows", path.display());
    out
}

/// The fixture files in this repo's spec directory, sorted so a run is
/// reproducible.
pub fn files() -> Vec<String> {
    let mut names: Vec<String> = fs::read_dir(spec_dir())
        .expect("the spec directory exists")
        .filter_map(|entry| entry.ok())
        .map(|entry| entry.file_name().to_string_lossy().into_owned())
        .filter(|name| name.ends_with(".tsv"))
        .collect();
    names.sort();
    // An empty list would run zero rows and `report` would then find no
    // failures, so the parity suite would go green having tested nothing.
    // A rename or a deletion under test/spec has to be loud.
    assert!(
        !names.is_empty(),
        "{} holds no .tsv fixtures; the parity suite would test nothing",
        spec_dir().display()
    );
    names
}

/// Report every failing row at once, naming file and line, rather than
/// stopping at the first.
pub fn report(failures: Vec<String>) {
    if failures.is_empty() {
        return;
    }
    panic!(
        "{} spec row(s) failed:\n{}",
        failures.len(),
        failures.join("\n")
    );
}

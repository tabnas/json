// The external conformance corpus, nst/JSONTestSuite, which
// `conformance_test.rs` grades and `reader_test.rs` reads from a reader.
//
// The suite is not vendored; it is fetched at a pinned commit into the
// .gitignore'd `test/jsontestsuite/` by `test/fetch-jsontestsuite.sh`,
// which `corpus()` below runs -- so a plain `cargo test`, locally and in
// CI, always reads it. If it cannot be obtained, what reads it FAILS
// rather than skips.

use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::OnceLock;

const MISSING: &str = "\
nst/JSONTestSuite corpus not found.

It is third-party and deliberately not vendored. Fetch it (pinned commit,
idempotent) and re-run:

    sh test/fetch-jsontestsuite.sh    # or: make json-test-suite

This fails rather than skips on purpose: a conformance suite that quietly
does not run reports a green tick that is a lie.";

fn repo_root() -> &'static Path {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("rs/ has a parent")
}

pub fn corpus_dir() -> PathBuf {
    repo_root()
        .join("test")
        .join("jsontestsuite")
        .join("test_parsing")
}

/// The sorted case file names.
///
/// Fetches the corpus first when it is not on disk, which is what Go's
/// `TestMain` does for `go test ./...`; Rust has no such hook, so the
/// fetch hangs off the first thing that needs the corpus and is run once
/// for the whole binary. If it still is not there, this panics with the
/// instructions rather than letting a test iterate nothing.
pub fn corpus() -> &'static Vec<String> {
    static CASES: OnceLock<Vec<String>> = OnceLock::new();
    CASES.get_or_init(|| {
        // UNCONDITIONALLY, even when the directory is already there.
        // The script is idempotent and touches the network only when it
        // has to, and its `corpus_ok` check compares the checkout's HEAD
        // against the PINNED SHA as well as the case counts. Skipping it
        // when the directory exists left the pin unverified: the census
        // `conformance_test.rs` asserts counts filenames, so a different
        // corpus with the same 95/188/35 shape would have graded green.
        let script = repo_root().join("test").join("fetch-jsontestsuite.sh");
        match Command::new("sh").arg(&script).status() {
            Ok(status) if status.success() => {}
            Ok(status) => eprintln!(
                "warning: {} exited {status}; the conformance tests will fail",
                script.display()
            ),
            Err(error) => eprintln!(
                "warning: could not run {} ({error}); the conformance tests will fail",
                script.display()
            ),
        }
        let dir = corpus_dir();

        let entries = fs::read_dir(&dir).unwrap_or_else(|_| panic!("{MISSING}"));
        let mut names: Vec<String> = entries
            .filter_map(|entry| entry.ok())
            .filter(|entry| entry.file_type().map(|t| t.is_file()).unwrap_or(false))
            .map(|entry| entry.file_name().to_string_lossy().into_owned())
            .filter(|name| name.ends_with(".json"))
            .collect();
        names.sort();
        assert!(!names.is_empty(), "{MISSING}");
        names
    })
}

pub fn cases(prefix: &'static str) -> impl Iterator<Item = &'static String> {
    corpus().iter().filter(move |name| name.starts_with(prefix))
}

pub fn source(name: &str) -> Vec<u8> {
    fs::read(corpus_dir().join(name)).unwrap_or_else(|e| panic!("read {name}: {e}"))
}

// The external RFC 8259 conformance suite.
//
// Runs nst/JSONTestSuite (https://github.com/nst/JSONTestSuite), the
// standard cross-implementation JSON parsing suite, against this crate.
// `ts/test/conformance.test.js` and `go/conformance_test.go` grade the
// SAME directory with the same rules, so the three runtimes cannot drift
// on it.
//
// The suite is not vendored; it is fetched at a pinned commit into the
// .gitignore'd `test/jsontestsuite/` by `test/fetch-jsontestsuite.sh`,
// which `corpus()` below runs when the corpus is absent -- so a plain
// `cargo test`, locally and in CI, always grades against it.
//
// If the corpus cannot be obtained these tests FAIL rather than skip. A
// conformance suite that quietly does not run reports a green tick that
// is a lie: it says "RFC 8259 conformant" while measuring nothing.
//
// The suite's file-name prefixes are the contract:
//
//   y_  MUST be accepted -- and must produce the same VALUE as the
//       platform parser, not merely "it did not error". The oracle,
//       serde_json, is independent of this crate, so the assertion is
//       not circular.
//   n_  MUST be rejected, with an error code.
//   i_  implementation-defined; this crate's contract is parity with the
//       platform parser, so the assertion is that the two agree.

mod common;

use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::OnceLock;

use common::oracle::{same, to_json};

// Exact shape of the corpus at the pinned commit
// (1ef36fa01286573e846ac449e8683f8833c5b26a); see
// test/fetch-jsontestsuite.sh. Asserted before grading, so narrowing the
// corpus goes red instead of inflating the pass rate.
const EXPECT_ACCEPT: usize = 95; // y_
const EXPECT_REJECT: usize = 188; // n_
const EXPECT_IMPL: usize = 35; // i_
const EXPECT_CASES: usize = 318;

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

fn corpus_dir() -> PathBuf {
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
/// instructions rather than letting the graders below iterate nothing.
fn corpus() -> &'static Vec<String> {
    static CASES: OnceLock<Vec<String>> = OnceLock::new();
    CASES.get_or_init(|| {
        // UNCONDITIONALLY, even when the directory is already there.
        // The script is idempotent and touches the network only when it
        // has to, and its `corpus_ok` check compares the checkout's HEAD
        // against the PINNED SHA as well as the case counts. Skipping it
        // when the directory exists left the pin unverified: the census
        // asserted below counts filenames, so a different corpus with the
        // same 95/188/35 shape would have graded green.
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

fn cases(prefix: &'static str) -> impl Iterator<Item = &'static String> {
    corpus().iter().filter(move |name| name.starts_with(prefix))
}

fn source(name: &str) -> Vec<u8> {
    fs::read(corpus_dir().join(name)).unwrap_or_else(|e| panic!("read {name}: {e}"))
}

/// This crate's parse, at the boundary a Rust caller actually has.
///
/// 25 of the 318 cases are not valid UTF-8, and `parse` takes a `&str`,
/// which cannot hold them: such a source is unrepresentable as an
/// argument, so a caller cannot submit it at all. That is a rejection one
/// step earlier than the parser, and it is reported as one. Go passes the
/// raw bytes in a `string` and TypeScript receives whatever Node's decode
/// produced, so this is where the three runtimes necessarily differ in
/// mechanism while agreeing on the verdict.
fn ours(src: &[u8]) -> Result<serde_json::Value, String> {
    let text = std::str::from_utf8(src).map_err(|e| format!("not valid UTF-8: {e}"))?;
    tabnas_json::parse(text)
        .map(|value| to_json(&value))
        .map_err(|error| {
            assert!(
                !error.code.is_empty(),
                "rejected without an error code: {error}"
            );
            error.code
        })
}

/// The oracle, given the same bytes. `from_slice` rejects invalid UTF-8
/// too, so the two sides are held to the same input rather than one of
/// them being handed a repaired copy.
fn oracle(src: &[u8]) -> Result<serde_json::Value, String> {
    serde_json::from_slice::<serde_json::Value>(src).map_err(|e| e.to_string())
}

/// The corpus is exactly the pinned one, so a half-cloned or narrowed
/// directory cannot pass vacuously.
#[test]
fn the_corpus_is_the_pinned_one() {
    let (y, n, i) = (
        cases("y_").count(),
        cases("n_").count(),
        cases("i_").count(),
    );
    assert!(
        y == EXPECT_ACCEPT
            && n == EXPECT_REJECT
            && i == EXPECT_IMPL
            && corpus().len() == EXPECT_CASES,
        "corpus has been narrowed or replaced under {}: \
         y_={y} n_={n} i_={i} total={}, want y_={EXPECT_ACCEPT} n_={EXPECT_REJECT} \
         i_={EXPECT_IMPL} total={EXPECT_CASES} -- \
         re-run `sh test/fetch-jsontestsuite.sh --force`",
        corpus_dir().display(),
        corpus().len()
    );
}

/// Every `y_` case parses, AND yields the value serde_json yields. "It did
/// not error" is not conformance: a parser that accepts the input and
/// returns the wrong value is silently losing data.
#[test]
fn must_accept() {
    let mut failures = Vec::new();
    for name in cases("y_") {
        let src = source(name);
        // If the oracle rejects a y_ case the corpus is wrong rather than
        // this crate, which is worth failing loudly for too.
        let want = match oracle(&src) {
            Ok(want) => want,
            Err(e) => {
                failures.push(format!("{name}: serde_json rejected a y_ case: {e}"));
                continue;
            }
        };
        match ours(&src) {
            Ok(got) if same(&got, &want) => {}
            Ok(got) => failures.push(format!("{name}: parse = {got}, serde_json = {want}")),
            Err(code) => failures.push(format!("{name}: must-accept case rejected: {code}")),
        }
    }
    report(failures);
}

/// Every `n_` case is rejected, with a code, and the oracle rejects it too.
#[test]
fn must_reject() {
    let mut failures = Vec::new();
    for name in cases("n_") {
        let src = source(name);
        if let Ok(value) = ours(&src) {
            failures.push(format!("{name}: must-reject case accepted: {value}"));
        }
        if oracle(&src).is_ok() {
            failures.push(format!("{name}: serde_json accepted an n_ case"));
        }
    }
    report(failures);
}

/// A case where this crate and serde_json genuinely differ, and exactly
/// how.
///
/// `why` alone is not enough. Recording only "these disagree" lets the
/// disagreement change shape unnoticed: a surrogate case could start
/// producing arbitrary text, or the number case start rejecting, and the
/// entry would still be satisfied. So each one pins the result.
struct Divergence {
    case: &'static str,
    why: &'static str,
    /// What this crate must produce, rendered as JSON. `None` means it
    /// must reject.
    ours: Option<&'static str>,
    /// Whether the oracle accepts it. Where both accept, the values
    /// differ; where only one does, that is the difference.
    oracle_accepts: bool,
}

/// The `i_` cases where this crate and serde_json genuinely differ.
///
/// Named one by one rather than skipped as a class, and the test below
/// checks each in BOTH directions: a listed case that starts agreeing
/// fails as a stale entry, and an unlisted case that starts differing
/// fails as a regression. With `ours` pinned, a listed case that keeps
/// disagreeing but in a NEW way fails too.
///
/// Every one of the 283 cases the RFC makes mandatory (`y_` and `n_`)
/// agrees; these 11 are inside the latitude the RFC leaves open.
const KNOWN_DIVERGENCES: &[Divergence] = &[
    // serde_json is the odd one out here, not the engine. The literal is
    // a 48-digit integer, and the engine's value is exactly what Rust's
    // own `str::parse::<f64>` returns, which is correctly rounded.
    // serde_json's big-integer path lands one ulp lower
    // (0x...c41 against 0x...c42). Matching it would mean deliberately
    // being less accurate than the standard library. Both ACCEPT; the
    // values differ, which is why the expected rendering is pinned to the
    // digit.
    Divergence {
        case: "i_number_very_big_negative_int",
        why: "serde_json is 1 ulp off str::parse::<f64>; the engine matches std",
        ours: Some("[-2.374623746732769e+47]"),
        oracle_accepts: true,
    },
    // A lone surrogate in a `\u` escape. serde_json rejects it outright;
    // the engine substitutes U+FFFD, which is what `encoding/json` does
    // (so the Go port agrees with ITS oracle) and close to what
    // `JSON.parse` does (so does the TypeScript one). Changing it would
    // mean this runtime's STRING DECODING diverging from the other two,
    // which is an engine-level decision in tabnas/parser rather than one
    // this grammar plugin can or should take on its own.
    //
    // The exact substitution is pinned per case: "it still disagrees" is
    // satisfied by any output at all, including a wrong one.
    Divergence {
        case: "i_object_key_lone_2nd_surrogate",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("{\"\u{FFFD}\":0.0}"),
        oracle_accepts: false,
    },
    Divergence {
        case: "i_string_1st_surrogate_but_2nd_missing",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("[\"\u{FFFD}\"]"),
        oracle_accepts: false,
    },
    Divergence {
        case: "i_string_1st_valid_surrogate_2nd_invalid",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("[\"\u{FFFD}\u{1234}\"]"),
        oracle_accepts: false,
    },
    Divergence {
        case: "i_string_incomplete_surrogate_and_escape_valid",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("[\"\u{FFFD}\\n\"]"),
        oracle_accepts: false,
    },
    Divergence {
        case: "i_string_incomplete_surrogate_pair",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("[\"\u{FFFD}a\"]"),
        oracle_accepts: false,
    },
    Divergence {
        case: "i_string_incomplete_surrogates_escape_valid",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("[\"\u{FFFD}\u{FFFD}\\n\"]"),
        oracle_accepts: false,
    },
    Divergence {
        case: "i_string_invalid_lonely_surrogate",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("[\"\u{FFFD}\"]"),
        oracle_accepts: false,
    },
    Divergence {
        case: "i_string_invalid_surrogate",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("[\"\u{FFFD}abc\"]"),
        oracle_accepts: false,
    },
    Divergence {
        case: "i_string_inverted_surrogates_U+1D11E",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("[\"\u{FFFD}\u{FFFD}\"]"),
        oracle_accepts: false,
    },
    Divergence {
        case: "i_string_lone_second_surrogate",
        why: "lone surrogate: U+FFFD, not a rejection",
        ours: Some("[\"\u{FFFD}\"]"),
        oracle_accepts: false,
    },
];

fn known(name: &str) -> Option<&'static Divergence> {
    let stem = name.strip_suffix(".json").unwrap_or(name);
    KNOWN_DIVERGENCES.iter().find(|d| d.case == stem)
}

/// Every `i_` case agrees with serde_json on accept versus reject, and on
/// the parsed value, except the ones named above -- which must diverge in
/// exactly the way they are named as diverging.
#[test]
fn implementation_defined() {
    let mut failures = Vec::new();
    for name in cases("i_") {
        let src = source(name);
        let ours = ours(&src);
        let oracle = oracle(&src);
        let agrees = match (&ours, &oracle) {
            (Ok(got), Ok(want)) => same(got, want),
            (Err(_), Err(_)) => true,
            _ => false,
        };

        let Some(known) = known(name) else {
            if !agrees {
                failures.push(format!(
                    "{name}: {}\n  (a NEW divergence from the platform oracle; \
                     fix it, or add it to KNOWN_DIVERGENCES with the reason \
                     and the expected result)",
                    describe(&ours, &oracle)
                ));
            }
            continue;
        };

        if agrees {
            failures.push(format!(
                "{name}: now agrees with serde_json, but is still listed in \
                 KNOWN_DIVERGENCES as {:?}; remove the entry",
                known.why
            ));
            continue;
        }

        // It still diverges. The entry says HOW, so check that too: a
        // divergence that changed shape is a regression the register
        // would otherwise absorb.
        let got = ours.as_ref().ok().map(|value| value.to_string());
        if got.as_deref() != known.ours {
            failures.push(format!(
                "{name}: diverges as expected, but not in the recorded way.\n  \
                 recorded: {:?}\n  actual:   {:?}\n  ({})",
                known.ours, got, known.why
            ));
        }
        if oracle.is_ok() != known.oracle_accepts {
            failures.push(format!(
                "{name}: the oracle now {} it, where the entry records {}",
                if oracle.is_ok() { "accepts" } else { "rejects" },
                if known.oracle_accepts {
                    "accept"
                } else {
                    "reject"
                }
            ));
        }
    }
    report(failures);
}

/// A one-line account of how the two sides differed, for a failure message.
fn describe(
    ours: &Result<serde_json::Value, String>,
    oracle: &Result<serde_json::Value, String>,
) -> String {
    match (ours, oracle) {
        (Ok(got), Ok(want)) => format!("parse = {got}, serde_json = {want}"),
        (Ok(got), Err(e)) => format!("parse accepted ({got}), serde_json rejected: {e}"),
        (Err(code), Ok(want)) => format!("parse rejected ({code}), serde_json = {want}"),
        (Err(a), Err(b)) => format!("both rejected ({a} / {b})"),
    }
}

/// Every failing case at once, named, rather than stopping at the first.
fn report(failures: Vec<String>) {
    if failures.is_empty() {
        return;
    }
    panic!(
        "{} conformance case(s) failed:\n{}",
        failures.len(),
        failures.join("\n")
    );
}

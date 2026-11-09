// A parse from a reader (`Tabnas::parse_reader`, admin ADR-30). `make()`
// declares `lex.window`, since this grammar's tokens are the engine's own
// matchers' and its one check reads no further than a number's literal, so
// a reader is read a window at a time. Every row of the shared fixtures and
// every document of the conformance corpus parses the same from a reader,
// handed over a few bytes at a time and under windows whose margin and
// block are a few characters (`parse_reader_with_sizes`, which makes every
// matcher meet the window's edge), as from its text, and so do long
// documents. `ts/test/reader.test.js` and `go/reader_test.go` hold the
// other runtimes to the same.

mod common;

use std::io::Read;

use common::corpus::{corpus, source};
use common::spec;
use tabnas::{Tabnas, WindowSizes};

/// The pieces a reader hands its bytes over in.
const PIECES: [usize; 3] = [1, 3, 64];

/// Windows whose margin and block are a few characters.
const WINDOWS: [(usize, usize); 3] = [(1, 1), (3, 2), (64, 64)];

/// A reader that hands its bytes over at most `size` at a time.
struct Pieces<'a> {
    bytes: &'a [u8],
    at: usize,
    size: usize,
}

impl Read for Pieces<'_> {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        let n = self.size.min(buf.len()).min(self.bytes.len() - self.at);
        buf[..n].copy_from_slice(&self.bytes[self.at..self.at + n]);
        self.at += n;
        Ok(n)
    }
}

fn pieces(bytes: &[u8], size: usize) -> Pieces<'_> {
    Pieces { bytes, at: 0, size }
}

fn outcome(parsed: Result<tabnas::Value, tabnas::TabnasError>) -> String {
    match parsed {
        Ok(value) => format!("ok {value}"),
        Err(e) => format!("err {} at {} {}:{}", e.code, e.pos, e.row, e.col),
    }
}

/// Every way this test reads `bytes` from a reader, each with its outcome:
/// in every size of pieces under the window's own sizes, and under every
/// small window.
fn readings(parser: &Tabnas, bytes: &[u8]) -> Vec<(String, String)> {
    let mut out = Vec::new();
    for size in PIECES {
        out.push((
            format!("pieces of {size}"),
            outcome(parser.parse_reader(pieces(bytes, size))),
        ));
        for (margin, block) in WINDOWS {
            let sizes = WindowSizes {
                margin,
                block,
                chunk: size,
            };
            out.push((
                format!("pieces of {size}, margin {margin}, block {block}"),
                outcome(parser.parse_reader_with_sizes(pieces(bytes, size), sizes)),
            ));
        }
    }
    out
}

/// A document of tokens each longer than a window's margin of 64 KiB, so
/// that a window grows to hold one: a string of escapes and characters past
/// ASCII, a number, a run of spaces and tabs, and a run of lines.
/// `go/reader_test.go`, which cannot make a window small, places a token of
/// every kind across the window's boundaries instead.
fn long_tokens() -> String {
    format!(
        "[\"{}\",1.{},{}{}true]",
        "ab\\u00e9ñ€𝄞\\n".repeat(5000),
        "1234567890".repeat(7000),
        " \t".repeat(40000),
        "\n\r\n".repeat(30000),
    )
}

#[test]
fn make_reads_a_reader_a_window_at_a_time() {
    assert!(tabnas_json::make().options.lex.window);
    // The plugin leaves the window to the parser that installs it, since
    // other grammars install it beneath matchers of their own.
    let mut parser = Tabnas::new();
    tabnas_json::json(&mut parser).expect("the JSON grammar installs");
    assert!(!parser.options.lex.window);
}

#[test]
fn every_row_parses_the_same_from_a_reader() {
    let parser = tabnas_json::make();
    let mut compared = 0;
    for file in spec::files() {
        for row in spec::rows(&file) {
            let whole = outcome(parser.parse(&row.input));
            for (how, read) in readings(&parser, row.input.as_bytes()) {
                assert_eq!(read, whole, "{}:{} in {how}", row.file, row.line);
                compared += 1;
            }
        }
    }
    assert!(compared > 1000, "{compared}");
}

#[test]
fn every_corpus_document_parses_the_same_from_a_reader() {
    // A document that is not UTF-8 is no text a reader can give: a reader
    // is refused where its bytes stop being UTF-8, `unreadable`, or at the
    // document's own error, the one the text before that point gives,
    // where the parse meets it first. Every other document reads as its
    // text does. The two longer than 4 KiB, 100,000 and 250,000 open
    // brackets, are read in pieces of 4 KiB.
    let parser = tabnas_json::make();
    let (mut compared, mut refused, mut long) = (0, 0, 0);
    for (index, name) in corpus().iter().enumerate() {
        let bytes = source(name);
        let reads = if bytes.len() > 4096 {
            long += 1;
            vec![(
                "pieces of 4096".to_string(),
                outcome(parser.parse_reader(pieces(&bytes, 4096))),
            )]
        } else {
            readings(&parser, &bytes)
        };
        match std::str::from_utf8(&bytes) {
            Ok(text) => {
                let whole = outcome(parser.parse(text));
                for (how, read) in reads {
                    assert_eq!(read, whole, "{name} in {how}");
                    compared += 1;
                }
            }
            Err(e) => {
                let before = std::str::from_utf8(&bytes[..e.valid_up_to()]).expect("UTF-8");
                let own = outcome(parser.parse(before));
                let mut unreadable: Vec<String> = Vec::new();
                for (how, read) in reads {
                    if read.starts_with("err unreadable ") {
                        if !unreadable.contains(&read) {
                            unreadable.push(read);
                        }
                    } else {
                        assert_eq!(read, own, "{name} in {how}");
                    }
                }
                assert!(unreadable.len() <= 1, "{name}: {unreadable:?}");
                refused += 1;
            }
        }
        if (index + 1) % 100 == 0 || index + 1 == corpus().len() {
            println!(
                "reader corpus: {} of {} ({}%)",
                index + 1,
                corpus().len(),
                (index + 1) * 100 / corpus().len()
            );
        }
    }
    assert_eq!(corpus().len(), 318);
    assert_eq!(refused, 25);
    assert_eq!(long, 2);
    assert!(compared > 3000, "{compared}");
}

/// A long document of `count` records, one a line.
fn records(count: usize) -> String {
    let records: Vec<String> = (0..count)
        .map(|i| format!("{{\"id\":{i},\"name\":\"ñ{i}\",\"tags\":[\"a\",\"b\"],\"n\":-1.5e3}}"))
        .collect();
    format!("[{}]", records.join(",\n"))
}

#[test]
fn a_long_document_parses_the_same_from_a_reader() {
    let parser = tabnas_json::make();
    let text = records(20_000);
    let whole = outcome(parser.parse(&text));
    assert!(whole.starts_with("ok "), "{whole}");
    let read = outcome(parser.parse_reader(pieces(text.as_bytes(), 4096)));
    assert_eq!(read, whole);
    // Under a small window, 2,000 records, some 110 KB, since a debug build
    // takes seconds over each parse of the 20,000.
    let text = records(2_000);
    let whole = outcome(parser.parse(&text));
    let small = WindowSizes {
        margin: 16,
        block: 16,
        chunk: 7,
    };
    let cut = outcome(parser.parse_reader_with_sizes(pieces(text.as_bytes(), 5), small));
    assert_eq!(cut, whole);
}

#[test]
fn tokens_longer_than_the_window_parse_the_same_from_a_reader() {
    // As written, with an error after them, and cut inside the string.
    let parser = tabnas_json::make();
    let text = long_tokens();
    let cut: String = text.chars().take(50_000).collect();
    let docs = [
        ("as written", text.clone(), "ok "),
        (
            "with an error after them",
            format!("{},]", &text[..text.len() - 1]),
            "err ",
        ),
        ("cut inside the string", cut, "err "),
    ];
    for (what, doc, kind) in docs {
        let whole = outcome(parser.parse(&doc));
        let head: String = whole.chars().take(80).collect();
        assert!(whole.starts_with(kind), "{what}: {head}");
        for size in [4096, 64 * 1024 + 7] {
            let read = outcome(parser.parse_reader(pieces(doc.as_bytes(), size)));
            assert_eq!(read, whole, "{what}, in pieces of {size}");
        }
        let small = WindowSizes {
            margin: 16,
            block: 16,
            chunk: 7,
        };
        let read = outcome(parser.parse_reader_with_sizes(pieces(doc.as_bytes(), 5), small));
        assert_eq!(read, whole, "{what}, in a small window");
    }
}

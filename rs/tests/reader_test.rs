// A parse from a reader (`Tabnas::parse_reader`, admin ADR-30). `make()`
// declares `lex.window`, since this grammar's tokens are the engine's own
// matchers' and its one check reads no further than a number's literal, so
// a reader is read a window at a time. Every row of the shared fixtures
// parses the same from a reader, handed over a few bytes at a time, as from
// its text, and so does a long document.

mod common;

use std::io::Read;

use common::spec;

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

fn outcome(parsed: Result<tabnas::Value, tabnas::TabnasError>) -> String {
    match parsed {
        Ok(value) => format!("ok {value}"),
        Err(e) => format!("err {} at {} {}:{}", e.code, e.pos, e.row, e.col),
    }
}

#[test]
fn make_reads_a_reader_a_window_at_a_time() {
    assert!(tabnas_json::make().options.lex.window);
}

#[test]
fn every_row_parses_the_same_from_a_reader() {
    let parser = tabnas_json::make();
    let mut compared = 0;
    for file in spec::files() {
        for row in spec::rows(&file) {
            let whole = outcome(parser.parse(&row.input));
            for size in [1, 3, 64] {
                let reader = Pieces {
                    bytes: row.input.as_bytes(),
                    at: 0,
                    size,
                };
                let read = outcome(parser.parse_reader(reader));
                assert_eq!(read, whole, "{}:{} in pieces of {size}", row.file, row.line);
                compared += 1;
            }
        }
    }
    assert!(compared > 100, "{compared}");
}

#[test]
fn a_long_document_parses_the_same_from_a_reader() {
    let records: Vec<String> = (0..20_000)
        .map(|i| format!("{{\"id\":{i},\"name\":\"ñ{i}\",\"tags\":[\"a\",\"b\"],\"n\":-1.5e3}}"))
        .collect();
    let text = format!("[{}]", records.join(",\n"));
    let parser = tabnas_json::make();
    let whole = outcome(parser.parse(&text));
    assert!(whole.starts_with("ok "), "{whole}");
    let reader = Pieces {
        bytes: text.as_bytes(),
        at: 0,
        size: 4096,
    };
    assert_eq!(outcome(parser.parse_reader(reader)), whole);
}

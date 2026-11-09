// Copyright (c) 2026 tabnas, MIT License

package tabnasjson

// reader_test.go — a parse from a reader (ParseReader, admin ADR-30).
//
// Make declares Lex.Window, since every token of this grammar is the
// engine's own matchers' and its one check reads no further than a
// number's text, so a reader is read a window at a time. Every row of the
// shared fixtures and every document of the conformance corpus parses the
// same from a reader, handed over a few bytes at a time, as from its text:
// the same value, or an error with the same code, position, row and
// column. The engine's hook that makes a window a few bytes is unexported
// in Go, so the window here is its own: 64 KiB of margin, block and read.
// A long document places a token of every kind across those boundaries,
// the tokens longer than the window included, and is handed over in
// pieces of several sizes. ts/test/reader.test.js and
// rs/tests/reader_test.rs hold the other runtimes to the same, under
// windows of a few units too.

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	tabnas "github.com/tabnas/parser/go"
	support "github.com/tabnas/support/go"
)

// readerPieces is a reader that hands its bytes over at most size at a time.
type readerPieces struct {
	bytes    []byte
	at, size int
}

func (p *readerPieces) Read(buf []byte) (int, error) {
	if p.at == len(p.bytes) {
		return 0, io.EOF
	}
	n := min(p.size, len(buf), len(p.bytes)-p.at)
	copy(buf, p.bytes[p.at:p.at+n])
	p.at += n
	return n, nil
}

func inPieces(text string, size int) io.Reader {
	return &readerPieces{bytes: []byte(text), size: size}
}

// render is a value as text, the keys of an object in their order, a
// number with its type and its sign, so that two values compare as text.
func render(v any) string {
	switch t := v.(type) {
	case *tabnas.OrderedMap:
		var b strings.Builder
		b.WriteByte('{')
		for i, k := range t.Keys {
			if 0 < i {
				b.WriteByte(',')
			}
			val, _ := t.Get(k)
			b.WriteString(strconv.Quote(k) + ":" + render(val))
		}
		b.WriteByte('}')
		return b.String()
	case []any:
		parts := make([]string, len(t))
		for i, val := range t {
			parts[i] = render(val)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case string:
		return strconv.Quote(t)
	default:
		return fmt.Sprintf("%T(%v)", t, t)
	}
}

// readOutcome is what a parse gave: its value, or its error's code,
// position, row and column, with the position the diagnostic counts in
// runes beside the error's own. An error's excerpt is left out: a long
// line's may be cut where the window begins, the one thing a parse from a
// reader shows differently.
func readOutcome(t *testing.T, value any, err error) string {
	t.Helper()
	if err == nil {
		return "ok " + render(value)
	}
	var je *tabnas.TabnasError
	if !errors.As(err, &je) {
		return "error " + err.Error()
	}
	raw, merr := stdjson.Marshal(je)
	if merr != nil {
		t.Fatalf("diagnostic: %v", merr)
	}
	var d struct {
		Code string `json:"code"`
		Pos  int    `json:"pos"`
	}
	if merr := stdjson.Unmarshal(raw, &d); merr != nil {
		t.Fatalf("diagnostic: %v", merr)
	}
	return fmt.Sprintf("err %s at %d (%d) %d:%d", je.Code, je.Pos, d.Pos, je.Row, je.Col)
}

// readerRecords is a long document of count records, one a line, as
// rs/tests/reader_test.rs writes it.
func readerRecords(count int) string {
	records := make([]string, count)
	for i := range records {
		records[i] = fmt.Sprintf(`{"id":%d,"name":"ñ%d","tags":["a","b"],"n":-1.5e3}`, i, i)
	}
	return "[" + strings.Join(records, ",\n") + "]"
}

// window64 is the window's margin, its block and a read: 64 KiB.
const window64 = 64 << 10

// straddlers are the tokens straddling places across the window's
// boundaries, by kind. The long ones are longer than the window's margin.
var straddlers = []struct{ kind, token string }{
	{"string", `"ab\u00e9\ud83d\ude00\n\"\\ ñ€𝄞 cd"`},
	{"number", `-1234.5678e-90`},
	{"true", `true`},
	{"false", `false`},
	{"null", `null`},
	{"characters past ASCII", `"ññ€€𝄞𝄞"`},
	{"an escape", `"\ud83d\ude00"`},
	{"space", strings.Repeat(" ", 9)},
	{"lines", "\r\n\n\r\n"},
	{"long string", `"` + strings.Repeat(`ab\u00e9ñ€𝄞\n`, 5000) + `"`},
	{"long number", "1." + strings.Repeat("1234567890", 7000)},
	{"long space", strings.Repeat(" \t", 40000)},
	{"long lines", strings.Repeat("\n\r\n", 30000)},
}

// straddling is a long document in which a token of every kind spans a
// multiple of 64 KiB: each kind in turn is placed across the next
// multiple, its middle on it, with records of every kind between. Where
// the token is characters past ASCII, the multiple falls inside one of
// them, so a read of 64 KiB splits it. Positions are bytes.
func straddling(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	b.WriteByte('[')
	first := true
	record := 0
	next := func() string {
		record++
		return fmt.Sprintf(`{"id":%d,"s":"x\ty ñ€𝄞","n":[-0.5,1e3,0],"t":true,"f":false,"z":null}`, record)
	}
	for _, s := range straddlers {
		half := len(s.token) / 2
		// The first multiple with room before it for a comma and a space.
		boundary := window64 * ((b.Len() + half + 2 + window64 - 1) / window64)
		for r := next(); b.Len()+1+len(r)+half+2 < boundary; r = next() {
			if !first {
				b.WriteByte(',')
			}
			b.WriteString(r)
			first = false
		}
		if strings.TrimSpace(s.token) != "" {
			if !first {
				b.WriteByte(',')
			}
			first = false
		}
		b.WriteString(strings.Repeat(" ", boundary-half-b.Len()))
		start := b.Len()
		b.WriteString(s.token)
		// Each token spans its multiple: begun before it, ended after it.
		if !(start < boundary && boundary < b.Len()) {
			t.Fatalf("%s: %d..%d does not span %d", s.kind, start, b.Len(), boundary)
		}
	}
	b.WriteString("," + next() + "]")
	return b.String()
}

func TestReaderMakeReadsAReaderAWindowAtATime(t *testing.T) {
	if !Make().Config().Window {
		t.Fatal("Make does not declare Lex.Window")
	}
	// The plugin leaves the window to the parser that installs it, since
	// other grammars install it beneath matchers of their own.
	j := tabnas.Make()
	if err := Json(j, nil); err != nil {
		t.Fatal(err)
	}
	if j.Config().Window {
		t.Fatal("the Json plugin declares Lex.Window")
	}
}

func TestReaderEveryRowParsesTheSameFromAReader(t *testing.T) {
	dir, err := support.FindSpecDir("")
	if err != nil {
		t.Fatal(err)
	}
	files, err := support.LoadSpecDir(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	j := Make()
	compared := 0
	for _, file := range files {
		for _, row := range file.Rows {
			input := row.UnescNamed("input")
			value, err := j.Parse(input)
			whole := readOutcome(t, value, err)
			for _, size := range []int{1, 3, 64} {
				value, err := j.ParseReader(inPieces(input, size))
				if read := readOutcome(t, value, err); read != whole {
					t.Fatalf("%s in pieces of %d:\n  read  %s\n  whole %s", row.Where(), size, read, whole)
				}
				compared++
			}
		}
	}
	if compared < 300 {
		t.Fatalf("only %d parses compared", compared)
	}
}

func TestReaderEveryCorpusDocumentParsesTheSameFromAReader(t *testing.T) {
	// A document that is not UTF-8 is no text a reader can give: a reader
	// is refused where its bytes stop being UTF-8, unreadable, or at the
	// document's own error where the parse meets that first. Every other
	// document reads as its text does. The two longer than 4 KiB, 100,000
	// and 250,000 open brackets, are read in pieces of 4 KiB.
	names := loadConformance(t)
	j := Make()
	compared, refused, long := 0, 0, 0
	for index, name := range names {
		bytes, err := os.ReadFile(filepath.Join(conformanceDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(bytes)
		value, err := j.Parse(text)
		whole := readOutcome(t, value, err)
		sizes := []int{1, 3, 64}
		if 4096 < len(bytes) {
			sizes = []int{4096}
			long++
		}
		unreadable := map[string]bool{}
		for _, size := range sizes {
			value, err := j.ParseReader(inPieces(text, size))
			read := readOutcome(t, value, err)
			if !utf8.Valid(bytes) && strings.HasPrefix(read, "err unreadable ") {
				unreadable[read] = true
				continue
			}
			if read != whole {
				t.Fatalf("%s in pieces of %d:\n  read  %s\n  whole %s", name, size, read, whole)
			}
			compared++
		}
		if !utf8.Valid(bytes) {
			if 1 < len(unreadable) {
				t.Fatalf("%s: unreadable at more than one place: %v", name, unreadable)
			}
			refused++
		}
		if (index+1)%100 == 0 || index+1 == len(names) {
			t.Logf("reader corpus: %d of %d (%d%%)", index+1, len(names), (index+1)*100/len(names))
		}
	}
	if refused != 25 || long != 2 || compared < 800 {
		t.Fatalf("%d documents not UTF-8 (want 25), %d longer than 4 KiB (want 2), %d parses compared",
			refused, long, compared)
	}
}

func TestReaderALongDocumentParsesTheSameFromAReader(t *testing.T) {
	j := Make()
	text := readerRecords(20000)
	value, err := j.Parse(text)
	whole := readOutcome(t, value, err)
	if !strings.HasPrefix(whole, "ok ") {
		t.Fatalf("%.80s", whole)
	}
	value, err = j.ParseReader(inPieces(text, 4096))
	if read := readOutcome(t, value, err); read != whole {
		t.Fatal("a long document parses differently from a reader")
	}
}

func TestReaderTokensAcrossTheWindowsBoundariesParseTheSameFromAReader(t *testing.T) {
	// As written, with an error after its last boundary, and cut inside
	// its long string.
	j := Make()
	text := straddling(t)
	if len(text) < 8*window64 {
		t.Fatalf("only %d bytes", len(text))
	}
	long := strings.Index(text, `"ab\u00e9ñ`)
	docs := []struct{ what, text, kind string }{
		{"as written", text, "ok "},
		{"with an error at its end", text[:len(text)-1] + ",x]", "err "},
		{"cut inside its long string", text[:long+50000], "err "},
	}
	for _, doc := range docs {
		value, err := j.Parse(doc.text)
		whole := readOutcome(t, value, err)
		if !strings.HasPrefix(whole, doc.kind) {
			t.Fatalf("%s: %.80s", doc.what, whole)
		}
		// Pieces of 64 and 4 KiB, and of 1 MiB, which a read takes 64 KiB at
		// a time, put a read's end on every multiple; 64 KiB and 7 does not.
		for _, size := range []int{64, 4096, window64 + 7, 1 << 20} {
			value, err := j.ParseReader(inPieces(doc.text, size))
			if read := readOutcome(t, value, err); read != whole {
				t.Fatalf("%s, in pieces of %d:\n  read  %.200s\n  whole %.200s", doc.what, size, read, whole)
			}
		}
		t.Logf("reader straddling: %s: %.40s", doc.what, whole)
	}
}

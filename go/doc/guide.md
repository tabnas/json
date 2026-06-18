# How-to guide: `tabnasjson` recipes (Go)

Task-oriented recipes for real problems. Each section is self-contained.
For a guided introduction read [`tutorial.md`](tutorial.md); for the full
API see [`reference.md`](reference.md); for the design see
[`concepts.md`](concepts.md).

The package name is `tabnasjson` (import path
`github.com/tabnas/json/go`); the engine is `tabnas` (import path
`github.com/tabnas/parser/go`).

## Parse a string and use the result

`Parse` returns the standard `encoding/json` Go types: `nil`, `bool`,
`float64`, `string`, `[]any`, and `map[string]any`. Type-assert to read
the value.

```go
v, err := tabnasjson.Parse(`{"id":7,"items":["pen","pad"],"paid":true}`)
if err != nil {
	return err
}
order := v.(map[string]any)
order["id"]               // float64(7)
order["items"].([]any)[1] // "pad"
order["paid"]             // true
```

## Handle invalid input

Invalid JSON returns a `*tabnas.TabnasError`. Use `errors.As` and read the
structured fields rather than scraping the message:

```go
import (
	"errors"

	tabnasjson "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

func tryParse(src string) (any, string, bool) {
	v, err := tabnasjson.Parse(src)
	if err == nil {
		return v, "", true
	}
	var je *tabnas.TabnasError
	if errors.As(err, &je) {
		return nil, je.Code, false // je.Code, je.Row, je.Col locate the error
	}
	return nil, "", false
}
```

The three codes this parser emits are `unexpected`,
`unterminated_string`, and `invalid_unicode`. These codes are part of the
parity contract shared with the TypeScript version.

## Reuse a parser efficiently

The top-level `Parse` already reuses a single lazily-built engine, so
repeated calls do not rebuild the grammar — and it is safe for concurrent
use because each parse builds its own context. When you need a
*configured* parser, build one with `Make` and keep it around:

```go
p := tabnasjson.Make()
a, _ := p.Parse(`{"x":1}`) // map[string]any{"x": 1}
b, _ := p.Parse(`{"y":2}`) // map[string]any{"y": 2}
```

## Keep introspection metadata (info options)

By default the parser produces plain values. To know whether a container
was explicit or to capture a string's quote character, enable the engine's
`Info` options through `Make`. The parsed values then come back wrapped in
the engine's info carriers — `MapRef`, `ListRef`, and `Text`:

```go
import (
	tabnasjson "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

tr := true
p := tabnasjson.Make(tabnas.Options{
	Info: &tabnas.InfoOptions{Map: &tr, List: &tr, Text: &tr},
})

out, _ := p.Parse(`{"a":["x",1]}`)

mr := out.(tabnas.MapRef) // mr.Implicit == false (explicit braces)
lr := mr.Val["a"].(tabnas.ListRef)
tx := lr.Val[0].(tabnas.Text) // tx.Quote == `"`, tx.Str == "x"
```

`MapRef.Implicit` / `ListRef.Implicit` record whether the container was
written explicitly; `Text.Quote` records the quote character. This is the
mode downstream plugins use when they must preserve syntax detail.

## Build a JSON-with-comments (JSONC) parser

The `Json` plugin is a foundation. `Make` with comment lexing re-enabled
yields a parser that accepts `//` and `/* */` comments:

```go
tr := true
jsonc := tabnasjson.Make(tabnas.Options{
	Comment: &tabnas.CommentOptions{Lex: &tr},
})

jsonc.Parse(`{"a":1} // trailing note`) // map[string]any{"a": 1}
jsonc.Parse(`{"a":/* inline */2}`)      // map[string]any{"a": 2}
```

This changes only your instance. The package-level `Parse` still rejects
comments.

## Install the grammar without the strict options

`Json` does two things: applies strict JSON lexer options *and* registers
the rule set. When you want only the rules — to extend them under your own
lexer configuration — call `RegisterJSONGrammar` directly on a bare
engine:

```go
import (
	tabnasjson "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

j := tabnas.Make()
if err := tabnasjson.RegisterJSONGrammar(j); err != nil {
	return err
}
v, _ := j.Parse(`{"a":[1,2,3]}`) // map[string]any{"a": []any{1, 2, 3}}
```

From here you can use the engine's rule API (`j.Rule(...)` and the
`@<rule>-<phase>` hooks) to extend the shared `val` / `map` / `list` /
`pair` / `elem` rules without redeclaring the JSON core.

## Pretty-print from the command line

The `tabnas-json` command parses and re-serializes with a 2-space indent
(via `json.MarshalIndent`).

```bash
# Build it once:
go build -o tabnas-json ./go/cmd/tabnas-json

# From an argument:
./tabnas-json '{"a":1,"b":[2,3]}'

# From stdin (a pipeline):
curl -s https://example.com/data.json | ./tabnas-json
```

On invalid input it writes the error to stderr and exits with code 1, so
it composes cleanly in shell pipelines.

## Note on numbers and key ordering

Like `encoding/json`, every JSON number parses to a `float64`, integers
included (`1` becomes `float64(1)`). And because Go maps are unordered, the
key order of a parsed object is not preserved — if you need order, use the
`Info` options and walk the structure, or parse with the metadata carriers
above.

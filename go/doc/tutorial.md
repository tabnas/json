# Tutorial: parsing your first JSON with `tabnasjson` (Go)

A learning-oriented walkthrough. You start with nothing and end with a
working Go program that parses JSON, handles an error, and extends the
parser. Follow it top to bottom.

This is the Go port of [`@tabnas/json`](../../ts/). For a reference of the
API read [`reference.md`](reference.md); for recipes read
[`guide.md`](guide.md); for design read [`concepts.md`](concepts.md).

The import path is `github.com/tabnas/json/go` and the package name is
`tabnasjson`.

## What you are building

`tabnasjson` is a standard JSON parser — it accepts exactly what
`encoding/json` accepts (RFC 8259 / ECMA-404) and nothing more. By the end
you will have used it to turn JSON text into Go values, caught a parse
error, and built a JSON-with-comments parser on top of it.

## Step 1 — Add the dependency

```bash
go get github.com/tabnas/json/go
```

The module depends on the `tabnas` engine, `github.com/tabnas/parser/go`.
Until that is published, it is resolved with a `replace` directive to a
sibling checkout — see the "Develop" section of [`../README.md`](../README.md).

## Step 2 — Parse a value

The whole library is reachable through one function: `Parse`. Give it a
JSON string; get back the Go value and an error.

```go
package main

import (
	"fmt"

	tabnasjson "github.com/tabnas/json/go"
)

func main() {
	v, err := tabnasjson.Parse(`42`)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%#v\n", v) // 42  (a float64)
}
```

`Parse` returns values using the same Go types as `encoding/json`: `nil`,
`bool`, `float64`, `string`, `[]any`, and `map[string]any`.

## Step 3 — Parse objects and arrays

The same `Parse` handles structured data. Nesting works to any depth.

```go
obj, _ := tabnasjson.Parse(`{"a":1,"b":2}`)
// obj is map[string]any{"a": 1, "b": 2}

arr, _ := tabnasjson.Parse(`[1, 2, 3]`)
// arr is []any{1, 2, 3}

nested, _ := tabnasjson.Parse(`{"a": {"b": [true, null]}}`)
// nested is map[string]any{"a": map[string]any{"b": []any{true, nil}}}
```

Insignificant whitespace between tokens is ignored, as in standard JSON.

## Step 4 — See what gets rejected

This parser is *strict*. Anything `encoding/json` would reject, it rejects
too. Each of these returns a non-nil error:

```go
tabnasjson.Parse(`{a:1}`)   // unquoted key
tabnasjson.Parse(`[1,2,]`)  // trailing comma
tabnasjson.Parse(`'x'`)     // single-quoted string
tabnasjson.Parse(`01`)      // leading zero
tabnasjson.Parse(`1 // hi`) // comment
```

That strictness is the point: `tabnasjson` is the baseline that relaxed
variants extend.

## Step 5 — Handle a parse error

Invalid input returns a `*tabnas.TabnasError`. Type-assert (or use
`errors.As`) to read its structured fields — `Code`, `Row`, `Col`:

```go
import (
	"errors"
	"fmt"

	tabnasjson "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

_, err := tabnasjson.Parse(`{a:1}`)
var je *tabnas.TabnasError
if errors.As(err, &je) {
	fmt.Println(je.Code) // unexpected
}
```

The `Error()` string is a human-readable, source-pointing message.

## Step 6 — Build your own parser instance

`Parse` uses one shared, lazily-built engine. When you want to customize
the parser, build your own instance with `Make`:

```go
p := tabnasjson.Make()
v, _ := p.Parse(`[1,2]`)
// v is []any{1, 2}
```

An instance is reusable and safe for concurrent use — build it once, call
`Parse` on it many times.

## Step 7 — Extend the grammar (JSON-with-comments)

Here is the foundation idea in action. The `Json` plugin installs the JSON
grammar onto a bare engine; `Make` lets you layer extra options. Turning
comment lexing back on turns strict JSON into JSONC:

```go
import (
	tabnasjson "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

tr := true
jsonc := tabnasjson.Make(tabnas.Options{
	Comment: &tabnas.CommentOptions{Lex: &tr},
})

jsonc.Parse(`{"a":1} // a note`)   // map[string]any{"a": 1}
jsonc.Parse(`{"a":/* inline */2}`) // map[string]any{"a": 2}
```

The package-level `Parse` still rejects those comments — you extended a
*new instance*, not the global parser.

## Step 8 — Use the command line

The module ships a tiny command, `tabnas-json`. It reads JSON from an
argument or from stdin and prints the re-serialized, pretty-printed form:

```bash
go run ./go/cmd/tabnas-json '{"a":1,"b":[2,3]}'
# {
#   "a": 1,
#   "b": [
#     2,
#     3
#   ]
# }

echo '[1,2,3]' | go run ./go/cmd/tabnas-json
```

Invalid input prints the error to stderr and exits with code 1.

## Where to go next

- [`guide.md`](guide.md) — task-focused recipes.
- [`reference.md`](reference.md) — the exact API and CLI surface.
- [`concepts.md`](concepts.md) — how the grammar-plugin model works, plus
  the differences from the TypeScript version.

# json (Go)

A standard JSON parser (RFC 8259 / ECMA-404) for Go. Strict by design —
no extended grammar, no dependencies beyond the standard library.

This is the Go port of [`@tabnas/json`](../ts/). TypeScript is canonical;
both runtimes share the conformance fixtures in
[`../ts/test/spec/`](../ts/test/spec/) and produce identical results.

## Install

```bash
go get github.com/tabnas/json/go
```

## Quick example

```go
package main

import (
	"fmt"

	json "github.com/tabnas/json/go"
)

func main() {
	v, err := json.Parse(`{"a":1,"b":[2,3]}`)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%#v\n", v) // map[string]interface {}{"a":1, "b":[]interface {}{2, 3}}
}
```

`Parse` returns values using the same Go types as `encoding/json`:
`nil`, `bool`, `float64`, `string`, `[]any`, and `map[string]any`.

## What it accepts

Exactly standard JSON — objects with double-quoted string keys, arrays,
double-quoted strings (with `\" \\ \/ \b \f \n \r \t \uXXXX` and surrogate
pairs), numbers, `true`, `false`, `null`, and insignificant whitespace.
It rejects everything outside that grammar (comments, trailing commas,
unquoted keys, single quotes, implicit structures, hex numbers, leading
zeros, empty input) — the same surface as `encoding/json`.

## Errors

On invalid input `Parse` returns a `*JsonError`:

```go
v, err := json.Parse("{a:1}")
if err != nil {
	var je *json.JsonError
	if errors.As(err, &je) {
		fmt.Println(je.Code)   // expected_key
		fmt.Println(je.Line)   // 1
		fmt.Println(je.Column) // 2
	}
}
```

## Develop

```bash
go build ./...
go test ./...   # also runs the shared ../ts/test/spec fixtures
```

See [`AGENTS.md`](AGENTS.md) for layout and conventions.

## License

MIT.

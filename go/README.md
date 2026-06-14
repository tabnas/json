# json (Go)

A standard JSON parser (RFC 8259 / ECMA-404) for Go — the standard-JSON
**grammar plugin** for the [`tabnas`](https://github.com/tabnas/parser)
parsing engine (`github.com/tabnas/parser/go`).

This is the Go port of [`@tabnas/json`](../ts/). TypeScript is canonical;
both runtimes share the conformance fixtures in
[`../ts/test/spec/`](../ts/test/spec/) and produce identical results.

## Install

```bash
go get github.com/tabnas/json/go
```

The module depends on `github.com/tabnas/parser/go`; until that is
published it is resolved via a `replace` directive to a sibling checkout
— see [Develop](#develop).

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

## Use it as a plugin

The package is a `tabnas` grammar plugin. Install it on your own engine
instance and layer further grammar on the shared rules:

```go
import (
	json "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

j := tabnas.Make()
j.Use(json.Json)
v, err := j.Parse(`{"a":[1,2,3]}`)
```

`RegisterJSONGrammar(j)` installs just the `val` / `map` / `list` /
`pair` / `elem` rules (jsonic's "Plain JSON" core) for plugins that want
to build on the JSON rule set.

## Reuse and options

`Parse` reuses a single lazily-created instance (safe for concurrent use,
since each parse builds its own context), so you don't rebuild the grammar
on every call. To customize, build your own instance with `Make(extra ...Options)`:

```go
tr := true
p := json.Make(tabnas.Options{Info: &tabnas.InfoOptions{Map: &tr, List: &tr}})
p.Parse(`{"a":[1,2]}`)
```

## What it accepts

Exactly standard JSON — objects with double-quoted string keys, arrays,
double-quoted strings (with `\" \\ \/ \b \f \n \r \t \uXXXX` and surrogate
pairs), numbers, `true`, `false`, `null`, and insignificant whitespace.
It rejects everything outside that grammar (comments, trailing commas,
unquoted keys, single quotes, implicit structures, hex numbers, leading
zeros, `.5`, `1.`, `+1`, empty input) — the same surface as
`encoding/json`.

## Errors

On invalid input `Parse` returns a `*tabnas.TabnasError`:

```go
v, err := json.Parse("{a:1}")
if err != nil {
	var je *tabnas.TabnasError
	if errors.As(err, &je) {
		fmt.Println(je.Code) // unexpected
	}
}
```

## Develop

This module depends on the engine as a sibling checkout:

```bash
git clone https://github.com/tabnas/parser   # sibling of this repo
go test ./...   # replace directive resolves ../../parser/go;
                # also runs the shared ../ts/test/spec fixtures
```

See [`AGENTS.md`](AGENTS.md) for layout and conventions.

## License

MIT.

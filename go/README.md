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

	tabnasjson "github.com/tabnas/json/go"
)

func main() {
	v, err := tabnasjson.Parse(`{"a":1,"b":[2,3]}`)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%#v\n", v) // map[string]interface {}{"a":1, "b":[]interface {}{2, 3}}
}
```

`Parse` returns values using the same Go types as `encoding/json`:
`nil`, `bool`, `float64`, `string`, `[]any`, and `map[string]any`.

## Documentation

Full [Diátaxis](https://diataxis.fr) docs:

- [`doc/tutorial.md`](doc/tutorial.md) — learn it step by step.
- [`doc/guide.md`](doc/guide.md) — task-focused recipes.
- [`doc/reference.md`](doc/reference.md) — the exact API and CLI surface.
- [`doc/concepts.md`](doc/concepts.md) — how it works, including the
  differences from the TypeScript version.

TypeScript is canonical; its docs are in [`../ts/doc/`](../ts/doc/).

## Use it as a plugin

The package is a `tabnas` grammar plugin. Install it on your own engine
instance and layer further grammar on the shared rules:

```go
import (
	tabnasjson "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
)

j := tabnas.Make()
j.Use(tabnasjson.Json)
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
p := tabnasjson.Make(tabnas.Options{Info: &tabnas.InfoOptions{Map: &tr, List: &tr}})
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

## Extending the grammar

Because this is a plain grammar plugin on the shared engine, it is a
foundation to build other parsers on. Layer options or rules on top of
the JSON grammar. For example, a JSON-with-comments (JSONC) parser is just
the JSON grammar with comment lexing re-enabled:

```go
tr := true
jsonc := tabnasjson.Make(tabnas.Options{Comment: &tabnas.CommentOptions{Lex: &tr}})
jsonc.Parse(`{"a":1} // ok`)    // map[string]any{"a": 1}
jsonc.Parse(`{"a":/* ok */1}`)  // map[string]any{"a": 1}
```

For deeper changes, call `RegisterJSONGrammar(j)` to install just the
rules, then use the engine's rule API (`j.Rule(...)`, and the `ClearOpen`
/ `ClearClose` / `@<rule>-<phase>/replace` hooks) to replace or extend the
shared `val` / `map` / `list` / `pair` / `elem` rules without re-declaring
the JSON core.

## Errors

On invalid input `Parse` returns a `*tabnas.TabnasError`:

```go
v, err := tabnasjson.Parse("{a:1}")
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

## Grammar diagram

The grammar is identical across runtimes. Its railroad/syntax diagram
(generated from the live TS grammar with
[`@tabnas/railroad`](https://github.com/tabnas/railroad)) lives in the TS
docs: [`../ts/doc/grammar.svg`](../ts/doc/grammar.svg), with an ASCII
version in [`../ts/doc/grammar.txt`](../ts/doc/grammar.txt).

## License

MIT.

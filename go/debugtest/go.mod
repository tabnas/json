// Isolated module for the json + @tabnas/debug integration test.
// It is a SEPARATE module (its own go.mod), so the main module's
// `go test ./...` does not descend into it and stays self-contained —
// it has no dependency on the external debug tool. json's own CI runs
// it: the shared polyglot-ci `go` job builds and tests every module
// under go/, this one included, with the parser and debug siblings (in
// ci.yml's `deps`) cloned beside this repo.
module github.com/tabnas/json/go/debugtest

go 1.24.7

require (
	github.com/tabnas/debug/go v0.3.8
	github.com/tabnas/json/go v0.5.17
)

require github.com/tabnas/parser/go v0.12.11

replace github.com/tabnas/json/go => ../

replace github.com/tabnas/debug/go => ../../../debug/go

replace github.com/tabnas/parser/go => ../../../parser/go

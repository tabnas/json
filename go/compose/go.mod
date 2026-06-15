// Isolated module for the optional json + @tabnas/debug composition test.
// It is a SEPARATE module (its own go.mod), so the main module's
// `go test ./...` does not descend into it and stays self-contained —
// it has no dependency on the external debug tool. The compose-debug CI
// job runs `go test` here with the parser and debug siblings checked out.
module github.com/tabnas/json/go/compose

go 1.24.7

require (
	github.com/tabnas/debug/go v0.0.0
	github.com/tabnas/json/go v0.0.0
)

require github.com/tabnas/parser/go v0.0.0 // indirect

replace github.com/tabnas/json/go => ../

replace github.com/tabnas/debug/go => ../../../debug/go

replace github.com/tabnas/parser/go => ../../../parser/go

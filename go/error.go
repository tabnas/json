// Copyright (c) 2026 tabnas, MIT License

package json

import "fmt"

// JsonError is the structured error returned by Parse when the input is
// not valid standard JSON. Code is a stable, machine-readable identifier;
// Index, Line and Column locate the offending position in the source.
type JsonError struct {
	Code   string
	Detail string
	Index  int
	Line   int
	Column int
}

func (e *JsonError) Error() string {
	return fmt.Sprintf("[json/%s] %s (at line %d, column %d)", e.Code, e.Detail, e.Line, e.Column)
}

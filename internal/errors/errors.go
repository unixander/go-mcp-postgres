package errors

import "fmt"

type GroupNotFoundError struct{ Group string }

func (e *GroupNotFoundError) Error() string {
	return fmt.Sprintf("group %q not found", e.Group)
}

type ContourNotFoundError struct{ Contour string }

func (e *ContourNotFoundError) Error() string {
	return fmt.Sprintf("contour %q not found", e.Contour)
}

type QueryError struct {
	Group   string
	Contour string
	Host    string
	DB      string
	PGCode  string
	Cause   error
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("query error [%s/%s host=%s db=%s pg_code=%s]: %v",
		e.Group, e.Contour, e.Host, e.DB, e.PGCode, e.Cause)
}

func (e *QueryError) Unwrap() error { return e.Cause }

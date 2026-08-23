package machine

import "fmt"

const (
	CodeBadRequest        = "bad_request"
	CodeConflict          = "revision_conflict"
	CodeMigrationRequired = "migration_required"
	CodeNotFound          = "not_found"
	CodeInternal          = "internal_error"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func apiError(code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

func responseError(err error) *APIError {
	if typed, ok := err.(*Error); ok {
		return &APIError{Code: typed.Code, Message: typed.Message}
	}
	return &APIError{Code: CodeInternal, Message: err.Error()}
}

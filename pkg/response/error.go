package response

import (
	"fmt"
)

const (
	CodeUnauthorized = 401
)

// Error holds an error code, message and error itself
type Error struct {
	Code     int
	Message  interface{}
	Internal error
}

func NewError(code int, message interface{}) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

func (e *Error) SetInternal(err error) *Error {
	e.Internal = err
	return e
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d: %v", e.Code, e.Message)
}

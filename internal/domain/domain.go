package domain

import "fmt"

const SchemaVersion = 1
const Version = "0.1.0-alpha.1-dev"

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }
func Invalid(format string, args ...any) error {
	return &Error{"invalid_input", fmt.Sprintf(format, args...)}
}
func Unsupported(message string) error { return &Error{"unsupported", message} }

type Capability struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Detail   string `json:"detail"`
	Provider string `json:"provider"`
}

package contract

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const MaxErrorMessageBytes = 512
const (
	msgCancelled = "Operation cancelled"
	msgTimeout   = "Operation timed out"
	msgFailed    = "Operation failed"
)

func FromError(err error) Error {
	if err == nil {
		return Error{}
	}
	var d *domain.Error
	if errors.As(err, &d) && d != nil {
		return normalizedError(d.Code, d.Message)
	}
	// Legacy capture decoding already yields a contract error. Keep mapping idempotent.
	var mapped Error
	if errors.As(err, &mapped) {
		return normalizedError(mapped.Code, mapped.Message)
	}
	if errors.Is(err, context.Canceled) {
		return Error{Code: domain.CodeCancelled, Message: msgCancelled}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Error{Code: domain.CodeTimeout, Message: msgTimeout}
	}
	return Error{Code: domain.CodeFailed, Message: msgFailed}
}
func normalizedError(code, message string) Error {
	if !domain.IsErrorCode(code) {
		code = domain.CodeFailed
	}
	return Error{Code: code, Message: sanitize(message)}
}
func sanitize(s string) string {
	var out strings.Builder
	out.Grow(min(len(s), MaxErrorMessageBytes))
	for _, r := range s {
		if unicode.IsControl(r) {
			r = ' '
		}
		if out.Len()+utf8.RuneLen(r) > MaxErrorMessageBytes {
			break
		}
		out.WriteRune(r)
	}
	if out.Len() == 0 {
		return msgFailed
	}
	return out.String()
}

package contract_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestFromError(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want contract.Error
	}{
		{"nil", nil, contract.Error{}},
		{"idempotent", contract.Error{Code: "timeout", Message: "x"}, contract.Error{Code: "timeout", Message: "x"}},
		{"domain", &domain.Error{Code: "permission_required", Message: "x"}, contract.Error{Code: "permission_required", Message: "x"}},
		{"wrapped domain", fmt.Errorf("internal detail: %w", &domain.Error{Code: "permission_required", Message: "x"}), contract.Error{Code: "permission_required", Message: "x"}},
		{"unknown code", &domain.Error{Code: "made_up", Message: "x"}, contract.Error{Code: "failed", Message: "x"}},
		{"cancelled", context.Canceled, contract.Error{Code: "cancelled", Message: "Operation cancelled"}},
		{"wrapped cancelled", fmt.Errorf("internal detail: %w", context.Canceled), contract.Error{Code: "cancelled", Message: "Operation cancelled"}},
		{"deadline", context.DeadlineExceeded, contract.Error{Code: "timeout", Message: "Operation timed out"}},
		{"raw internal", errors.New("sql: password=synthetic-secret"), contract.Error{Code: "failed", Message: "Operation failed"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := contract.FromError(test.err); got != test.want {
				t.Fatalf("got=%+v, want=%+v", got, test.want)
			}
		})
	}
}

func TestFromErrorBoundsMessage(t *testing.T) {
	for _, test := range []struct{ name, input, want string }{
		{"at limit", strings.Repeat("x", 512), strings.Repeat("x", 512)},
		{"limit plus one", strings.Repeat("x", 513), strings.Repeat("x", 512)},
		{"rune boundary", strings.Repeat("x", 511) + "é", strings.Repeat("x", 511)},
		{"controls", "a\n\x1bb", "a  b"},
		{"empty", "", "Operation failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := contract.FromError(&domain.Error{Code: "failed", Message: test.input})
			if got.Message != test.want || len(got.Message) > contract.MaxErrorMessageBytes || !utf8.ValidString(got.Message) {
				t.Fatalf("message=%q, bytes=%d", got.Message, len(got.Message))
			}
		})
	}
}

func TestErrorImplementsErrorString(t *testing.T) {
	if got := (contract.Error{Code: "timeout", Message: "x"}).Error(); got != "timeout: x" {
		t.Fatal(got)
	}
}

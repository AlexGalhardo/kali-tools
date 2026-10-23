package localdiscovery

import (
	"strings"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// Secret redacts itself in every formatting path.
type Secret struct{ value string }

func NewSecret(s string) (Secret, error) {
	if s == "" || len(s) > MaxSecretBytes || strings.ContainsRune(s, 0) {
		return Secret{}, domain.Invalid("invalid secret")
	}
	return Secret{value: s}, nil
}

func (Secret) String() string { return "[REDACTED]" }

func (Secret) GoString() string { return "[REDACTED]" }

func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }

func (s Secret) reveal() string { return s.value }

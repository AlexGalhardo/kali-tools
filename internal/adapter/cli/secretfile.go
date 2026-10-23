package cli

import (
	"os"
	"strings"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/localdiscovery"
)

const maxSecretFileBytes = 4 << 10

func readSecretLines(path string) (string, string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", domain.Invalid("secret file must be a regular file")
	}
	if info.Size() > maxSecretFileBytes {
		return "", "", domain.Invalid("secret file is too large")
	}
	if err := secretFileOwner(info); err != nil {
		return "", "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", domain.Invalid("secret file must be a regular file")
	}
	parts := strings.Split(string(raw), "\n")
	line1 := strings.TrimRight(parts[0], "\r")
	line2 := ""
	if len(parts) > 1 {
		line2 = strings.TrimRight(parts[1], "\r")
	}
	if len(line1) > localdiscovery.MaxSecretBytes || len(line2) > localdiscovery.MaxSecretBytes {
		return "", "", domain.Invalid("invalid secret")
	}
	return line1, line2, nil
}

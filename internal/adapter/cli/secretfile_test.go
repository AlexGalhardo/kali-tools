package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestReadSecretLinesFromStdin(t *testing.T) {
	t.Cleanup(func() { secretInput = io.Reader(os.Stdin) })
	secretInput = strings.NewReader("synthetic-auth\r\nsynthetic-priv\n")
	a, b, err := readSecretLines("-")
	if err != nil || a != "synthetic-auth" || b != "synthetic-priv" {
		t.Fatalf("got %q %q %v", a, b, err)
	}
	secretInput = strings.NewReader(strings.Repeat("a", maxSecretFileBytes+1))
	if _, _, err := readSecretLines("-"); err == nil {
		t.Fatal("oversized stdin accepted")
	}
}

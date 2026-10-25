package tools

import "testing"

func TestResolveMissingBinary(t *testing.T) {
	if _, err := resolve("kali-tools-no-such-binary"); err == nil {
		t.Fatal("expected not found")
	}
}

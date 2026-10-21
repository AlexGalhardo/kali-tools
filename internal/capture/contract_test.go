package capture_test

import (
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
)

func TestLiveProviderContract(t *testing.T) {
	porttest.LiveProviderContract(t, func(*testing.T) capture.LiveProvider { return capture.Native{} })
}

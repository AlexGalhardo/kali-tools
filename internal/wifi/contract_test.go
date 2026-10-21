package wifi_test

import (
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

func TestWiFiProviderContract(t *testing.T) {
	porttest.WiFiProviderContract(t, func(*testing.T) app.WiFiProvider { return wifi.Native{} })
}

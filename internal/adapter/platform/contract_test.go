package platform_test

import (
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/platform"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
)

func TestHostNetworkContract(t *testing.T) {
	porttest.HostNetworkContract(t, func(*testing.T) app.HostNetwork { return platform.Host{} })
}

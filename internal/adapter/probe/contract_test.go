package probe_test

import (
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/probe"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
)

func TestProberContract(t *testing.T) {
	porttest.ProberContract(t, func(*testing.T) network.Prober { return probe.ICMP{} })
}

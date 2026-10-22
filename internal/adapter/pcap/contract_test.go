//go:build !(windows && pcap)

package pcap_test

import (
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/pcap"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
)

func TestNpcapStubContract(t *testing.T) {
	porttest.LiveProviderContract(t, func(*testing.T) capture.LiveProvider { return pcap.NewNpcap() })
}

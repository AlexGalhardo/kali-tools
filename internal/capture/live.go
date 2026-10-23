package capture

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/gopacket/gopacket/layers"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

type LiveRequest struct {
	Backend         string `json:"backend,omitempty"`
	Address         string `json:"address"`
	DurationSeconds int    `json:"duration_seconds"`
	PacketLimit     int    `json:"packet_limit"`
}
type LiveResult struct {
	Limited  bool      `json:"limited"`
	Packets  []Packet  `json:"packets"`
	Provider string    `json:"provider"`
	Dropped  *uint64   `json:"dropped"`
	Limits   []string  `json:"limits"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished"`
}
type LiveProvider interface {
	Capture(context.Context, LiveRequest) (LiveResult, *Artifact, error)
}

type Native struct{}

func (Native) Capture(ctx context.Context, request LiveRequest) (LiveResult, *Artifact, error) {
	return LiveSession(ctx, request)
}

func Live(ctx context.Context, r LiveRequest) (LiveResult, error) { return live(ctx, r, nil) }

func LiveSession(ctx context.Context, r LiveRequest) (LiveResult, *Artifact, error) {
	artifact := NewArtifact(layers.LinkTypeRaw)
	result, err := live(ctx, r, artifact)
	return result, artifact, err
}

func ValidateLiveRequest(r LiveRequest) error {
	if r.DurationSeconds < 1 || r.DurationSeconds > 60 || r.PacketLimit < 1 || r.PacketLimit > MaxPackets {
		return domain.Invalid("select a local non-loopback IPv4, duration 1..60 and packet limit 1..10000")
	}
	switch r.Backend {
	case BackendPktmon:
		if r.Address != "" {
			return domain.Invalid("pktmon captures all NICs; address must be empty")
		}
		return nil
	case "", BackendRawSocket, BackendNpcap, BackendLibpcap:
	default:
		return domain.Invalid("backend must be raw_socket, pktmon, npcap or libpcap")
	}
	ip, err := netip.ParseAddr(r.Address)
	if err != nil {
		return domain.Invalid("select a local non-loopback IPv4, duration 1..60 and packet limit 1..10000")
	}
	if r.Backend != BackendNpcap && r.Backend != BackendLibpcap && (!ip.Is4() || ip.IsLoopback()) {
		return domain.Invalid("select a local non-loopback IPv4, duration 1..60 and packet limit 1..10000")
	}
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}
	for _, a := range addresses {
		prefix, err := netip.ParsePrefix(a.String())
		if err == nil && prefix.Addr() == ip {
			return nil
		}
	}
	return domain.Invalid("capture address must belong to a local interface")
}

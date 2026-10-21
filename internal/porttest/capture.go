package porttest

import (
	"net"
	"net/netip"
	"reflect"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
)

func LiveProviderContract(t *testing.T, newProvider func(*testing.T) capture.LiveProvider) {
	t.Helper()
	t.Run("honest_state_without_permission", func(t *testing.T) {
		addresses, err := net.InterfaceAddrs()
		if err != nil {
			t.Fatal(err)
		}
		var address string
		for _, candidate := range addresses {
			prefix, err := netip.ParsePrefix(candidate.String())
			if err == nil && prefix.Addr().Is4() && !prefix.Addr().IsLoopback() {
				address = prefix.Addr().String()
				break
			}
		}
		if address == "" {
			t.Skip("no non-loopback IPv4")
		}
		request := capture.LiveRequest{Address: address, DurationSeconds: 1, PacketLimit: 1}
		v, _, err := newProvider(t).Capture(deadline(t), request)
		RequireHonestState(t, err, "unsupported", "permission_required")
		if err == nil && len(v.Packets) > request.PacketLimit {
			t.Fatal("packet limit exceeded")
		}
	})
	t.Run("result_carries_no_payload_field", func(t *testing.T) {
		for field := range reflect.TypeFor[capture.Packet]().Fields() {
			if field.Type.Kind() == reflect.Slice && field.Type.Elem().Kind() == reflect.Uint8 {
				t.Fatal("packet exposes raw bytes")
			}
		}
	})
}

package capture

import "testing"

func TestCaptureRequestBounds(t *testing.T) {
	for _, r := range []LiveRequest{
		{Address: "127.0.0.1", DurationSeconds: 1, PacketLimit: 1},
		{Address: "::1", DurationSeconds: 1, PacketLimit: 1},
		{Address: "invalid", DurationSeconds: 1, PacketLimit: 1},
		{Address: "192.0.2.1", DurationSeconds: 0, PacketLimit: 1},
		{Address: "192.0.2.1", DurationSeconds: 61, PacketLimit: 1},
		{Address: "192.0.2.1", DurationSeconds: 1, PacketLimit: 0},
		{Address: "192.0.2.1", DurationSeconds: 1, PacketLimit: MaxPackets + 1},
	} {
		if err := ValidateLiveRequest(r); err == nil {
			t.Fatalf("invalid capture request accepted: %+v", r)
		}
	}
}

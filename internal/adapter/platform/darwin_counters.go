package platform

import (
	"encoding/binary"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const (
	darwinIFINFO2     = 0x12
	darwinIbytesOff   = 96
	darwinObytesOff   = 104
	darwinMinIfInfo2  = 112
	maxDarwinMessages = 4096
)

// darwinCounter is one interface from an RTM_IFINFO2 message.
// Byte offsets match if_msghdr2: if_data64 begins at byte 32, and ifi_ibytes is 64 bytes into that struct.
type darwinCounter struct {
	Index    uint32
	Received uint64
	Sent     uint64
}

func parseDarwinCounters(raw []byte) ([]darwinCounter, error) {
	if len(raw) > maxNetlinkBytes {
		return nil, domain.Invalid("interface dump exceeds limit")
	}
	out := []darwinCounter{}
	seen := map[uint32]struct{}{}
	messages := 0
	for len(raw) > 0 {
		if len(raw) < 4 {
			return nil, domain.Invalid("truncated interface message")
		}
		size := int(binary.NativeEndian.Uint16(raw[:2]))
		if size < 4 || size > len(raw) {
			return nil, domain.Invalid("invalid interface message length")
		}
		messages++
		if messages > maxDarwinMessages {
			return nil, domain.Invalid("interface dump exceeds limit")
		}
		if raw[3] == darwinIFINFO2 && size >= darwinMinIfInfo2 {
			index := uint32(binary.NativeEndian.Uint16(raw[12:14]))
			if _, ok := seen[index]; index > 0 && !ok {
				seen[index] = struct{}{}
				out = append(out, darwinCounter{
					Index:    index,
					Received: binary.NativeEndian.Uint64(raw[darwinIbytesOff : darwinIbytesOff+8]),
					Sent:     binary.NativeEndian.Uint64(raw[darwinObytesOff : darwinObytesOff+8]),
				})
			}
		}
		advance := (size + 3) &^ 3
		if advance > len(raw) {
			advance = size
		}
		raw = raw[advance:]
	}
	return out, nil
}

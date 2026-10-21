package capture

import (
	"encoding/binary"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// Validate declared lengths before handing untrusted files to decoders that allocate.
func preflight(raw []byte) error {
	bad := func() error { return domain.Invalid("malformed or oversized capture record") }
	if len(raw) < 4 {
		return bad()
	}
	var order binary.ByteOrder = binary.LittleEndian
	if binary.LittleEndian.Uint32(raw[:4]) == 0x0a0d0d0a {
		for offset := 0; offset < len(raw); {
			if len(raw)-offset < 12 {
				return bad()
			}
			block := raw[offset:]
			if binary.LittleEndian.Uint32(block[:4]) == 0x0a0d0d0a {
				if len(block) < 28 {
					return bad()
				}
				switch binary.LittleEndian.Uint32(block[8:12]) {
				case 0x1a2b3c4d:
					order = binary.LittleEndian
				case 0x4d3c2b1a:
					order = binary.BigEndian
				default:
					return bad()
				}
			}
			n := int(order.Uint32(block[4:8]))
			if n < 12 || n%4 != 0 || n > 1<<20 || n > len(block) || order.Uint32(block[n-4:n]) != uint32(n) {
				return bad()
			}
			kind := order.Uint32(block[:4])
			switch kind {
			case 1:
				if n < 20 || order.Uint32(block[12:16]) > 1<<20 {
					return bad()
				}
			case 6:
				if n < 32 {
					return bad()
				}
				captured := int(order.Uint32(block[20:24]))
				if captured > 1<<20 || captured > n-32 {
					return bad()
				}
			case 2:
				if n < 32 || int(order.Uint32(block[20:24])) > n-32 {
					return bad()
				}
			case 3:
				if n < 16 || order.Uint32(block[8:12]) > 1<<20 {
					return bad()
				}
			}
			offset += n
		}
		return nil
	}
	if len(raw) < 24 {
		return bad()
	}
	switch binary.LittleEndian.Uint32(raw[:4]) {
	case 0xa1b2c3d4, 0xa1b23c4d:
		order = binary.LittleEndian
	case 0xd4c3b2a1, 0x4d3cb2a1:
		order = binary.BigEndian
	default:
		return bad()
	}
	snap := order.Uint32(raw[16:20])
	if snap == 0 || snap > 1<<20 {
		return bad()
	}
	for offset := 24; offset < len(raw); {
		if len(raw)-offset < 16 {
			return bad()
		}
		n := int(order.Uint32(raw[offset+8 : offset+12]))
		original := int(order.Uint32(raw[offset+12 : offset+16]))
		if n > int(snap) || n > len(raw)-offset-16 || n > original {
			return bad()
		}
		offset += 16 + n
	}
	return nil
}

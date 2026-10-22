package platform

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func neighborAttr(kind uint16, data []byte) []byte {
	raw := make([]byte, (4+len(data)+3)&^3)
	binary.NativeEndian.PutUint16(raw, uint16(4+len(data)))
	binary.NativeEndian.PutUint16(raw[2:], kind)
	copy(raw[4:], data)
	return raw
}

func neighborMessage(kind uint16, body []byte) []byte {
	raw := make([]byte, (16+len(body)+3)&^3)
	binary.NativeEndian.PutUint32(raw, uint32(16+len(body)))
	binary.NativeEndian.PutUint16(raw[4:], kind)
	copy(raw[16:], body)
	return raw
}

func neighborDump(family byte, state uint16, flags byte, address, mac []byte) []byte {
	body := make([]byte, 12, 12+len(address)+len(mac)+16)
	body[0], body[10] = family, flags
	binary.NativeEndian.PutUint32(body[4:], 7)
	binary.NativeEndian.PutUint16(body[8:], state)
	body = append(body, neighborAttr(1, address)...)
	body = append(body, neighborAttr(2, mac)...)
	return neighborMessage(28, body)
}

func TestParseNetlinkNeighbors(t *testing.T) {
	raw := neighborDump(2, 2, 0, []byte{192, 0, 2, 1}, []byte{2, 0, 0, 0, 0, 1})
	raw = append(raw, neighborDump(10, 4, 128, []byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, nil)...)
	raw = append(raw, neighborDump(2, 64, 0, []byte{192, 0, 2, 2}, nil)...)
	raw = append(raw, neighborMessage(99, nil)...)
	raw = append(raw, neighborMessage(3, nil)...)
	raw = append(raw, []byte("ignored after done")...)
	rows, err := parseNetlinkNeighbors(raw, map[uint32]string{7: "fixture"})
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	if rows[0].Address != "192.0.2.1" || rows[0].State != network.NeighborReachable || rows[0].MAC != "02:00:00:00:00:01" || rows[0].InterfaceName != "fixture" {
		t.Fatal(rows[0])
	}
	if rows[1].Address != "fe80::1%fixture" || rows[1].State != network.NeighborStale || !rows[1].Router || rows[1].MAC != "" {
		t.Fatal(rows[1])
	}
}

func TestParseNetlinkNeighborsMalformed(t *testing.T) {
	valid := neighborDump(2, 2, 0, []byte{192, 0, 2, 1}, nil)
	for _, test := range []struct {
		name string
		raw  []byte
		code string
	}{
		{"short header", []byte{1}, "invalid_input"},
		{"header length below header", make([]byte, 16), "invalid_input"},
		{"message over buffer", valid[:len(valid)-1], "invalid_input"},
		{"short ndmsg", neighborMessage(28, []byte{2}), "invalid_input"},
		{"bad destination", neighborDump(2, 2, 0, make([]byte, 5), nil), "invalid_input"},
		{"bad IPv6 destination", neighborDump(10, 2, 0, make([]byte, 4), nil), "invalid_input"},
		{"large MAC", neighborDump(2, 2, 0, []byte{192, 0, 2, 1}, make([]byte, 33)), "invalid_input"},
		{"large input", make([]byte, (4<<20)+1), "invalid_input"},
		{"kernel error", neighborMessage(2, nil), "failed"},
		{"message padding", neighborMessage(99, []byte{0})[:17], "invalid_input"},
		{"attribute padding", neighborMessage(28, append(make([]byte, 12), []byte{5, 0, 99, 0, 0}...)), "invalid_input"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseNetlinkNeighbors(test.raw, nil)
			var de *domain.Error
			if !errors.As(err, &de) || de.Code != test.code {
				t.Fatal(err)
			}
		})
	}
	for _, size := range []uint16{0, 3, 1000} {
		t.Run(fmt.Sprintf("attribute length %d", size), func(t *testing.T) {
			raw := append([]byte(nil), valid...)
			binary.NativeEndian.PutUint16(raw[28:], size)
			if _, err := parseNetlinkNeighbors(raw, nil); err == nil {
				t.Fatal("bad attribute accepted")
			}
		})
	}
	for _, n := range []int{1, 2, 3} {
		if _, err := parseNetlinkNeighbors(neighborMessage(28, append(make([]byte, 12), make([]byte, n)...)), nil); err == nil {
			t.Fatal("truncated attribute accepted")
		}
	}
}

func TestParseNetlinkNeighborStatesAndBounds(t *testing.T) {
	for _, test := range []struct {
		bits uint16
		want string
	}{{128 | 2, "permanent"}, {2 | 4, "reachable"}, {4 | 8, "stale"}, {8 | 16, "delay"}, {16 | 1, "probe"}, {1 | 32, "incomplete"}, {32, "unreachable"}, {0, "unknown"}} {
		t.Run(test.want, func(t *testing.T) {
			rows, err := parseNetlinkNeighbors(neighborDump(2, test.bits, 0, []byte{192, 0, 2, 1}, make([]byte, 32)), nil)
			if err != nil || len(rows) != 1 || rows[0].State != test.want || rows[0].MAC != "" {
				t.Fatal(rows, err)
			}
		})
	}
	for _, count := range []int{65536, 65537} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			raw := bytes.Repeat(neighborDump(2, 2, 0, []byte{192, 0, 2, 1}, nil), count)
			rows, err := parseNetlinkNeighbors(raw, nil)
			if err != nil || len(rows) != 65536 {
				t.Fatal(len(rows), err)
			}
		})
	}
	if rows, err := parseNetlinkNeighbors(neighborDump(99, 2, 0, []byte{1, 2, 3, 4}, nil), nil); err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	padding := make([]byte, 4<<20)
	copy(padding, neighborMessage(3, nil))
	if _, err := parseNetlinkNeighbors(padding, nil); err != nil {
		t.Fatal("exact input limit rejected", err)
	}
}

func TestParseProcNetARP(t *testing.T) {
	fixture := "IP address HW type Flags HW address Mask Device\n192.0.2.1 0x1 0x2 02:00:00:00:00:01 * fixture\n192.0.2.2 0x1 0x0 00:00:00:00:00:00 * fixture\n192.0.2.3 0x1 0x6 02:00:00:00:00:03 * missing\nmalformed line\n"
	rows, err := parseProcNetARP(strings.NewReader(fixture), map[string]uint32{"fixture": 7})
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	if rows[0].State != "unknown" || rows[1].State != "incomplete" || rows[2].State != "permanent" || rows[0].InterfaceIndex != 7 || rows[2].InterfaceIndex != 0 || rows[1].MAC != "" {
		t.Fatal(rows)
	}
	for _, size := range []int{1 << 20, (1 << 20) + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			_, err := parseProcNetARP(strings.NewReader(strings.Repeat("\n", size)), nil)
			if (err == nil) != (size == 1<<20) {
				t.Fatal(err)
			}
		})
	}
	if _, err := parseProcNetARP(failedNeighborReader{}, nil); err == nil {
		t.Fatal("reader failure ignored")
	}
	malformed := "header\ninvalid 0x1 0x2 02:00:00:00:00:01 * fixture\n::1 0x1 0x2 02:00:00:00:00:01 * fixture\n192.0.2.1 0x1 invalid 02:00:00:00:00:01 * fixture\n192.0.2.1 0x1 0x2 invalid * fixture\n"
	if rows, err := parseProcNetARP(strings.NewReader(malformed), nil); err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}

type failedNeighborReader struct{}

func (failedNeighborReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

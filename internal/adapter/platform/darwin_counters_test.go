package platform

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestParseDarwinCounters(t *testing.T) {
	raw := append(ifinfo2(2, 10, 20), ifinfo2(3, 1, 2)...)
	raw = append(raw, ifinfo2(2, 99, 99)...)
	got, err := parseDarwinCounters(raw)
	if err != nil || len(got) != 2 || got[0].Index != 2 || got[0].Received != 10 || got[0].Sent != 20 || got[1].Index != 3 {
		t.Fatalf("counters=%+v err=%v", got, err)
	}
	skipped := []byte{0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.NativeEndian.PutUint16(skipped[:2], 16)
	got, err = parseDarwinCounters(append(append([]byte{}, skipped...), ifinfo2(0, 5, 5)...))
	if err != nil || len(got) != 0 {
		t.Fatalf("skipped=%+v err=%v", got, err)
	}
	short := ifinfo2(4, 1, 1)
	binary.NativeEndian.PutUint16(short[:2], 20)
	got, err = parseDarwinCounters(short[:20])
	if err != nil || len(got) != 0 {
		t.Fatalf("short=%+v err=%v", got, err)
	}
	for _, raw := range [][]byte{{1, 0, 0}, {0, 0, 0, 0}, bytes.Repeat([]byte{1}, maxNetlinkBytes+1)} {
		_, err = parseDarwinCounters(raw)
		var invalid *domain.Error
		if !errors.As(err, &invalid) || invalid.Code != domain.CodeInvalidInput {
			t.Fatalf("raw len %d err=%v", len(raw), err)
		}
	}
}

func ifinfo2(index uint16, received, sent uint64) []byte {
	raw := make([]byte, 160)
	binary.NativeEndian.PutUint16(raw[:2], 160)
	raw[3] = darwinIFINFO2
	binary.NativeEndian.PutUint16(raw[12:14], index)
	binary.NativeEndian.PutUint64(raw[darwinIbytesOff:darwinIbytesOff+8], received)
	binary.NativeEndian.PutUint64(raw[darwinObytesOff:darwinObytesOff+8], sent)
	return raw
}

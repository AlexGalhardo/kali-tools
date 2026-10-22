package platform

import "testing"

func FuzzParseNetlinkNeighbors(f *testing.F) {
	f.Add([]byte{})
	f.Add(neighborDump(2, 2, 0, []byte{192, 0, 2, 1}, []byte{2, 0, 0, 0, 0, 1}))
	f.Add(neighborDump(10, 4, 128, make([]byte, 16), nil))
	f.Fuzz(func(t *testing.T, raw []byte) {
		rows, _ := parseNetlinkNeighbors(raw, map[uint32]string{7: "fixture"})
		if len(rows) > 65536 {
			t.Fatal("raw row limit exceeded")
		}
	})
}

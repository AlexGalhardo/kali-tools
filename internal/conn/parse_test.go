package conn

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseProcMapsFixtureProcessAndOmitsMissingInode(t *testing.T) {
	text := "" +
		"  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:1F90 0A0200C0:01BB 01 00000000:00000000 00:00000000 00000000     0        0 4242 1 0000000000000000 100 0 0 10 0\n" +
		"   1: 0100007F:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 9999 1 0000000000000000 100 0 0 10 0\n" +
		"   2: not-an-address\n"
	rows := parseNet(text, "tcp")
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	table := assemble(rows, map[uint64]owner{4242: {pid: 42, name: "fixture"}}, false)
	if table.Provider != "Linux procfs" || table.Limited || len(table.Connections) != 1 {
		t.Fatalf("%+v", table)
	}
	got := table.Connections[0]
	want := Connection{Protocol: "tcp", Local: "127.0.0.1:8080", Remote: "192.0.2.10:443", State: "established", PID: 42, Process: "fixture"}
	if got != want {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
	encoded, err := json.Marshal(table)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"received", "sent", "bits_per_second", "mbps"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("table contains %s: %s", forbidden, encoded)
		}
	}
}

func TestParseIPv6LoopbackListen(t *testing.T) {
	text := "   0: 00000000000000000000000001000000:0016 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 1000 0 5555 1 0000000000000000 100 0 0 10 0\n"
	rows := parseNet(text, "tcp6")
	if len(rows) != 1 || rows[0].local != "[::1]:22" || rows[0].state != "listen" || rows[0].inode != 5555 {
		t.Fatalf("%+v", rows)
	}
}

func TestAssembleKeepsLowerPIDAndCapsRows(t *testing.T) {
	raws := make([]rawRow, maxRows+2)
	owners := map[uint64]owner{}
	for i := range raws {
		raws[i] = rawRow{protocol: "tcp", local: "127.0.0.1:1", remote: "127.0.0.1:2", state: "established", inode: uint64(i + 1)}
		owners[uint64(i+1)] = owner{pid: 100, name: "a"}
	}
	owners[1] = owner{pid: 9, name: "first"}
	table := assemble(raws, owners, false)
	if !table.Limited || len(table.Connections) != maxRows {
		t.Fatalf("limited=%v len=%d", table.Limited, len(table.Connections))
	}
	if table.Connections[0].PID != 9 || table.Connections[0].Process != "first" {
		t.Fatalf("%+v", table.Connections[0])
	}
}

func FuzzParseNet(f *testing.F) {
	f.Add("  sl  local_address rem_address st\n   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 1 1 0 0 0 0 0\n")
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 1<<16 {
			text = text[:1<<16]
		}
		_ = parseNet(text, "tcp")
	})
}

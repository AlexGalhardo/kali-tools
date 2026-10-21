package tools

import "testing"

func TestNmapParser(t *testing.T) {
	result, e := ParseNmap([]byte(`<nmaprun><host><status state="up"/><address addr="192.0.2.1" addrtype="ipv4"/><ports><port portid="80" protocol="tcp"><state state="open"/></port></ports></host></nmaprun>`))
	if e != nil || len(result.Hosts) != 1 || result.Hosts[0].Ports[0].Port != 80 {
		t.Fatal(result, e)
	}
	if _, e = ParseNmap([]byte("<nmaprun>")); e == nil {
		t.Fatal("malformed XML accepted")
	}
}
func FuzzNmap(f *testing.F) {
	f.Add([]byte("<nmaprun/>"))
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = ParseNmap(data) })
}

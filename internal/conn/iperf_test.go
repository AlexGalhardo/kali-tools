package conn

import (
	"strings"
	"testing"
)

func TestParseIperfJSONIgnoresBandwidth(t *testing.T) {
	const report = `{"start":{"connecting_to":{"host":"192.0.2.10","port":5201}},"end":{"sum_sent":{"bytes":4096,"bits_per_second":999999,"seconds":1.5}}}`
	sample, err := ParseIperfJSON([]byte(report))
	if err != nil {
		t.Fatal(err)
	}
	if sample.Endpoint != "192.0.2.10:5201" || sample.ByteCount != 4096 || sample.DurationMS != 1500 || sample.Source != "iperf3" {
		t.Fatalf("%+v", sample)
	}
	encoded := sample.Endpoint + " " + sample.Source
	if strings.Contains(encoded, "999999") || strings.Contains(encoded, "bits_per_second") {
		t.Fatal(encoded)
	}
}

func TestParseIperfJSONRejectsGarbage(t *testing.T) {
	for _, body := range []string{"", "[]", `{"end":{"sum_sent":{"bytes":1}}}`} {
		if _, err := ParseIperfJSON([]byte(body)); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
}

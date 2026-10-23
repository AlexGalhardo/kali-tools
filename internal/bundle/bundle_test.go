package bundle

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestSecretFixturesNeverAppear(t *testing.T) {
	secrets := []string{
		"synthetic-psk-do-not-leak",
		"snmp-community-secret",
		"PAYLOAD-BYTES-SECRET",
		"secret-192.0.2.55-key",
	}
	planted := strings.Join(secrets, " ")
	note := "router 192.0.2.10 and 192.0.2.10 via 2001:db8::10 mac AA:BB:CC:DD:EE:FF ssid=CafeWiFi hostname=lab-host.internal at 2026-10-23T18:40:00.123456789Z see https://npcap.com/#download " + planted
	body, err := bytesOf(Input{
		AppVersion:  "0.6.0-dev.2",
		GoVersion:   "go1.27.3",
		OS:          "linux",
		Arch:        "amd64",
		Hostname:    "lab-host.internal",
		Names:       []string{"CafeWiFi", "Office WiFi"},
		Secrets:     secrets,
		CaptureNote: note,
		WiFiNote:    "Office WiFi " + planted,
		Capabilities: []Capability{{
			ID: "wifi", State: "available", Detail: note, Provider: "fixture",
		}},
		Tools:    []Tool{{Name: "tshark", State: "available", Version: planted, Path: "/opt/" + planted + "/tshark"}},
		Settings: Settings{Language: "en", RetentionDays: 30},
		Logs: []Log{{
			ID: "0123456789abcdef0123456789abcdef", Kind: "tcp", State: "completed",
			Summary: note, Created: "2026-10-23T18:40:00.123456789Z",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || len(body) > MaxBundleBytes {
		t.Fatalf("size=%d", len(body))
	}
	text := unzipText(t, body)
	for _, secret := range secrets {
		if bytes.Contains(body, []byte(secret)) || strings.Contains(text, secret) {
			t.Fatalf("secret %q appeared", secret)
		}
	}
	for _, literal := range []string{"192.0.2.10", "192.0.2.55", "2001:db8::10", "AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff", "CafeWiFi", "Office WiFi", "lab-host.internal", "npcap.com"} {
		if strings.Contains(text, literal) {
			t.Fatalf("identifier %q appeared in %s", literal, text)
		}
	}
	for _, token := range []string{"ip1", "mac1", "name1", "2026-10-23T18:40:00.123456789Z", "Nothing in this archive was uploaded.", "schema_version"} {
		if !strings.Contains(text, token) {
			t.Fatalf("missing %q in %s", token, text)
		}
	}
	if strings.Count(text, "ip1") < 2 {
		t.Fatalf("same address was not stable: %s", text)
	}
	if strings.Contains(text, `"payload"`) || strings.Contains(text, `"psk"`) || strings.Contains(text, `"secret"`) || strings.Contains(text, `"packet"`) {
		t.Fatalf("report has a forbidden key: %s", text)
	}
}

func TestLogCapAndEmptyCollections(t *testing.T) {
	logs := make([]Log, MaxLogs+3)
	for i := range logs {
		logs[i] = Log{ID: "id", Kind: "tcp", State: "completed", Summary: "ok", Created: "2026-10-23T18:40:00Z"}
	}
	body, err := bytesOf(Input{AppVersion: "0.6.0-dev.2", OS: "linux", Arch: "amd64", Settings: Settings{Language: "en", RetentionDays: 30}, Logs: logs})
	if err != nil {
		t.Fatal(err)
	}
	text := unzipText(t, body)
	if strings.Contains(text, "logs: 53") || !strings.Contains(text, "logs: 50") {
		t.Fatalf("log cap not applied: %s", text)
	}
	if !strings.Contains(text, `"capabilities": []`) || !strings.Contains(text, `"tools": []`) {
		t.Fatalf("empty collections were null: %s", text)
	}
}

func unzipText(t *testing.T, body []byte) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 || reader.File[0].Name != "summary.txt" || reader.File[1].Name != "report.json" {
		t.Fatalf("files=%v", names(reader))
	}
	var b strings.Builder
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, MaxBundleBytes))
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		b.Write(content)
		b.WriteByte('\n')
	}
	return b.String()
}

func names(reader *zip.Reader) []string {
	out := make([]string, len(reader.File))
	for i, file := range reader.File {
		out[i] = file.Name
	}
	return out
}

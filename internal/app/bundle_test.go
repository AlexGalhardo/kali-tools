package app_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/porttest/fake"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

type countingWiFi struct{ current, reveal int }

func (c *countingWiFi) Current(context.Context) (wifi.Network, error) {
	c.current++
	return wifi.Network{SSID: "CafeWiFi", BSSID: "aa:bb:cc:dd:ee:ff"}, nil
}
func (c *countingWiFi) Reveal(context.Context) (wifi.Secret, error) {
	c.reveal++
	return wifi.Secret{State: "available", Value: "synthetic-psk-do-not-leak"}, nil
}

func TestDiagnosticBundleOmitsSecretsAndQueries(t *testing.T) {
	store := fake.NewStore()
	created := time.Date(2026, 10, 23, 18, 40, 0, 123456789, time.UTC)
	if err := store.PutJob(t.Context(), job.Status{
		ID: "0123456789abcdef0123456789abcdef", Kind: "tcp", State: job.Completed, Created: created,
		Summary: "saw 192.0.2.10 aa:bb:cc:dd:ee:ff ssid=CafeWiFi hostname=lab-host.internal",
	}); err != nil {
		t.Fatal(err)
	}
	provider := &countingWiFi{}
	application := app.New(app.Deps{Jobs: store, Settings: store, WiFi: provider, Tools: fake.Tools{}})
	t.Cleanup(func() { _ = application.Close() })
	if _, err := application.RevealWiFi(t.Context()); err != nil {
		t.Fatal(err)
	}
	body, err := application.DiagnosticBundle(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if provider.current != 0 || provider.reveal != 1 {
		t.Fatalf("bundle queried wifi current=%d reveal=%d", provider.current, provider.reveal)
	}
	text := unzip(t, body)
	for _, literal := range []string{"synthetic-psk-do-not-leak", "192.0.2.10", "aa:bb:cc:dd:ee:ff", "CafeWiFi", "lab-host.internal", "PAYLOAD-BYTES-SECRET"} {
		if strings.Contains(text, literal) {
			t.Fatalf("%q leaked into %s", literal, text)
		}
	}
	if !strings.Contains(text, "Nothing in this archive was uploaded.") || !strings.Contains(text, "ip1") || !strings.Contains(text, "name1") {
		t.Fatalf("bundle=%s", text)
	}
}

func unzip(t *testing.T, body []byte) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		b.Write(content)
	}
	return b.String()
}

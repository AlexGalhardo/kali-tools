package cli_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
)

func TestDoctorBundleWritesLocalZip(t *testing.T) {
	application := newApp(t)
	path := filepath.Join(t.TempDir(), "diag.zip")
	var out, stderr bytes.Buffer
	code := cli.Run(t.Context(), application, []string{"--json", "doctor", "--bundle", path}, &out, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%s out=%s", code, stderr.String(), out.String())
	}
	var envelope struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data["path"] != path || envelope.Data["format"] != "zip" {
		t.Fatalf("data=%v", envelope.Data)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v err=%v", info, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("files=%d", len(reader.File))
	}
	rc, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	summary, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil || !bytes.Contains(summary, []byte("Nothing in this archive was uploaded.")) {
		t.Fatalf("summary=%s err=%v", summary, err)
	}
	if bytes.Contains(body, []byte("synthetic-psk")) || strings.Contains(out.String(), "upload") {
		t.Fatal("bundle response leaked a secret or an upload")
	}
}

func TestDoctorBundleRejectsADirectory(t *testing.T) {
	var out, stderr bytes.Buffer
	code := cli.Run(t.Context(), newApp(t), []string{"doctor", "--bundle", t.TempDir()}, &out, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "not writable") {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
}

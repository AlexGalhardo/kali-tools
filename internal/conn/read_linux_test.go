//go:build linux

package conn

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestReadProcMapsOwnerAndSkipsUnreadableProcess(t *testing.T) {
	root := t.TempDir()
	writeProcFile(t, filepath.Join(root, "net", "tcp"), ""+
		"  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"+
		"   0: 0100007F:1F90 0A0200C0:01BB 01 00000000:00000000 00:00000000 00000000 0 0 4242 1 0000000000000000 100 0 0 10 0\n"+
		"   1: 0100007F:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 9999 1 0000000000000000 100 0 0 10 0\n"+
		"   2: 0100007F:0400 00000000:0000 0A 00000000:00000000 00:00000000 00000000 0 0 7777 1 0000000000000000 100 0 0 10 0\n")
	writeProcFile(t, filepath.Join(root, "42", "comm"), "fixture\n")
	link(t, filepath.Join(root, "42", "fd"), "3", "socket:[4242]")
	writeProcFile(t, filepath.Join(root, "7", "comm"), "hidden\n")
	if err := os.MkdirAll(filepath.Join(root, "7", "fd"), 0o755); err != nil {
		t.Fatal(err)
	}
	link(t, filepath.Join(root, "7", "fd"), "4", "socket:[7777]")
	if err := os.Chmod(filepath.Join(root, "7", "fd"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(root, "7", "fd"), 0o755) })

	table, err := readProc(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Connections) != 1 {
		t.Fatalf("%+v", table.Connections)
	}
	got := table.Connections[0]
	if got.PID != 42 || got.Process != "fixture" || got.Local != "127.0.0.1:8080" || got.Remote != "192.0.2.10:443" {
		t.Fatalf("%+v", got)
	}
}

func TestReadProcPermissionAndMissingTable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses mode bits")
	}
	root := t.TempDir()
	writeProcFile(t, filepath.Join(root, "net", "tcp"), "sl\n")
	if err := os.Chmod(filepath.Join(root, "net", "tcp"), 0); err != nil {
		t.Fatal(err)
	}
	_, err := readProc(t.Context(), root)
	var target *domain.Error
	if !errors.As(err, &target) || target.Code != domain.CodePermissionRequired || target.Message == "" {
		t.Fatal(err)
	}
	missing := t.TempDir()
	if err := os.Mkdir(filepath.Join(missing, "net"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = readProc(t.Context(), missing)
	if !errors.As(err, &target) || target.Code != domain.CodeUnsupported {
		t.Fatal(err)
	}
}

func TestReadSeesOwnListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	table, err := Read(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := "127.0.0.1:" + port
	for _, row := range table.Connections {
		if row.Local == want && row.PID == os.Getpid() && row.State == "listen" && row.Protocol == "tcp" && row.Process != "" {
			return
		}
	}
	t.Fatalf("listener %s pid %d missing from %d rows", want, os.Getpid(), len(table.Connections))
}

func TestReadLimitedRejectsOversizedTable(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "tcp")
	body := make([]byte, maxFileBytes+1)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := readLimited(path)
	var target *domain.Error
	if !errors.As(classify(err), &target) || target.Code != domain.CodeFailed {
		t.Fatal(err)
	}
}

func writeProcFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func link(t *testing.T, dir, name, target string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
}

func TestSocketInode(t *testing.T) {
	inode, ok := socketInode("socket:[4242]")
	if !ok || inode != 4242 {
		t.Fatal(inode, ok)
	}
	if _, ok := socketInode("pipe:[1]"); ok {
		t.Fatal("pipe")
	}
	if strconv.Itoa(int(inode)) != "4242" {
		t.Fatal(inode)
	}
}

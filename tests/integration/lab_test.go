//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func TestPrivateLab(t *testing.T) {
	host := os.Getenv("LAB_FIXTURE")
	if host == "" {
		t.Fatal("LAB_FIXTURE is required; use docker compose lab")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil || len(ips) == 0 {
		t.Fatal(ips, e)
	}
	address := ips[0].IP.String()
	httpResult, e := (network.Diagnostics{}).Diagnose(ctx, network.Request{Kind: "http", Target: "http://" + net.JoinHostPort(address, "8080") + "/health", TimeoutMS: 1000})
	if e != nil || httpResult.Values[0] != "204 No Content" {
		t.Fatal(httpResult, e)
	}
	inventory, e := network.Discover(ctx, address+"/32", []int{8080, 9090, 9091}, 500*time.Millisecond)
	if e != nil || len(inventory) != 3 {
		t.Fatal(inventory, e)
	}
	for _, r := range inventory {
		want := "open"
		if r.Port == 9091 {
			want = "unreachable"
		}
		if r.State != want {
			t.Fatalf("port %d: %s", r.Port, r.State)
		}
	}
	original := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", net.JoinHostPort(address, "5353"))
	}}
	defer func() { net.DefaultResolver = original }()
	result, e := (network.Diagnostics{}).Diagnose(ctx, network.Request{Kind: "dns", Target: "fixture.test", TimeoutMS: 1000})
	if e != nil || strings.Join(result.Values, ",") != "192.0.2.42" {
		t.Fatal(result, e)
	}
	_, e = (network.Diagnostics{}).Diagnose(ctx, network.Request{Kind: "dns", Target: "missing.test", TimeoutMS: 1000})
	if dns, ok := e.(*net.DNSError); !ok || !dns.IsNotFound {
		t.Fatal("expected NXDOMAIN", strconv.Quote(result.Target), e)
	}
}

func TestCLIExecutable(t *testing.T) {
	host := os.Getenv("LAB_FIXTURE")
	if host == "" {
		t.Fatal("LAB_FIXTURE is required")
	}
	for _, c := range []struct {
		args     []string
		code     int
		expected string
	}{
		{[]string{"version"}, 0, "version"},
		{[]string{"diagnose", "--kind", "http", "--target", "http://" + net.JoinHostPort(host, "8080") + "/health"}, 0, "204 No Content"},
		{[]string{"diagnose", "--target", "invalid;host"}, 2, "invalid_input"},
		{[]string{"discover", "--target", "0.0.0.0/0"}, 2, "invalid_input"},
	} {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "kali-tools", c.args...)
		cmd.WaitDelay = time.Second
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		cancel()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		if code != c.code {
			t.Fatalf("%v: exit %d: %s", c.args, code, stderr.String())
		}
		var response struct {
			SchemaVersion int `json:"schema_version"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &response); err != nil || response.SchemaVersion != 1 || !strings.Contains(stdout.String(), c.expected) {
			t.Fatalf("%v: %s: %v", c.args, stdout.String(), err)
		}
	}
}

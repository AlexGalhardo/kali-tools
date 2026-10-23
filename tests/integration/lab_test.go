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
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func TestPrivateLabNeighbors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires the Linux Docker lab with all capabilities dropped")
	}
	host := os.Getenv("LAB_FIXTURE")
	if host == "" {
		t.Fatal("LAB_FIXTURE is required; use the private Docker lab")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil || len(addresses) == 0 {
		t.Fatal("fixture IPv4 lookup failed", err)
	}
	address := addresses[0].String()
	_, err = (network.Diagnostics{}).Diagnose(ctx, network.Request{Kind: "http", Target: "http://" + net.JoinHostPort(address, "8080") + "/health", TimeoutMS: 1000})
	if err != nil {
		t.Fatal("fixture contact failed", err)
	}
	a, cleanup, err := bootstrap.New(ctx, bootstrap.Config{DataPath: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			t.Error(err)
		}
	}()
	var out, stderr bytes.Buffer
	if code := cli.Run(ctx, a, []string{"neighbors", "--family", "ipv4", "--json"}, &out, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	var response struct {
		SchemaVersion int                    `json:"schema_version"`
		Data          contract.NeighborTable `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil || response.SchemaVersion != 1 || response.Data.Provider != "Linux netlink RTM_GETNEIGH" {
		t.Fatal("invalid neighbor response", err)
	}
	for _, row := range response.Data.Neighbors {
		if row.Address == address {
			return
		}
	}
	t.Fatal("contacted fixture is absent from the passive neighbor cache")
}

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
		return (&net.Dialer{}).DialContext(ctx, "udp", net.JoinHostPort(address, "5300"))
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
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
		{[]string{"wifi", "reveal"}, 2, "invalid_input"},
		{[]string{"diagnose", "--kind", "http", "--target", "http://" + net.JoinHostPort(host, "8080") + "/health"}, 0, "204 No Content"},
		{[]string{"diagnose", "--target", "invalid;host"}, 2, "invalid_input"},
		{[]string{"discover", "--target", "0.0.0.0/0"}, 2, "invalid_input"},
	} {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		cmd := exec.CommandContext(ctx, "kali-tools", append([]string{"--json"}, c.args...)...)
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

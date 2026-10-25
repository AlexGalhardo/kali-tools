package acceptance

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
)

func openPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener.Addr().(*net.TCPAddr).Port
}
func DiscoverySpec(t *testing.T, newDriver scenarioDriver) {
	t.Helper()
	t.Run("finds_open_loopback_port", func(t *testing.T) {
		d := newDriver(t)
		port := openPort(t)
		v, err := d.Discover(testContext(t), "127.0.0.1/32", []int{port}, 500*time.Millisecond)
		for i := range v {
			v[i].RTTMS = 0
		}
		if err != nil || !reflect.DeepEqual(v, []contract.PortResult{{Host: "127.0.0.1", Port: port, State: "open", HostState: "up"}}) {
			t.Fatalf("result=%v err=%v", v, err)
		}
	})
	t.Run("rejects_wide_scope", func(t *testing.T) {
		for _, cidr := range []string{"0.0.0.0/0", "10.0.0.0/23"} {
			t.Run(cidr, func(t *testing.T) {
				d := newDriver(t)
				v, err := d.Discover(testContext(t), cidr, []int{80}, 500*time.Millisecond)
				assertCode(t, err, "invalid_input")
				if len(v) != 0 {
					t.Fatal(v)
				}
			})
		}
	})
	t.Run("port_count_bounds", func(t *testing.T) {
		for _, count := range []int{32, 33} {
			name := "at_limit"
			if count == 33 {
				name = "over_limit"
			}
			t.Run(name, func(t *testing.T) {
				d := newDriver(t)
				ports := make([]int, count)
				for i := range ports {
					ports[i] = i + 1
				}
				v, err := d.Discover(testContext(t), "127.0.0.1/32", ports, 500*time.Millisecond)
				if count == 33 {
					assertCode(t, err, "invalid_input")
				} else if err != nil || len(v) != 32 {
					t.Fatalf("results=%d err=%v", len(v), err)
				}
			})
		}
	})
}

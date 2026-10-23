package porttest

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/app"
)

func ToolRunnerContract(t *testing.T, newRunner func(*testing.T) app.ToolRunner) {
	t.Helper()
	t.Run("inventory_lists_known_tools", func(t *testing.T) {
		runner := newRunner(t)
		all := runner.Inventory(deadline(t))
		var names []string
		for _, tool := range all {
			names = append(names, tool.Name)
			if !slices.Contains([]string{"available", "dependency_missing", "failed", "unsupported"}, tool.State) {
				t.Fatalf("tool state=%s", tool.State)
			}
			if tool.State != "available" && tool.Version != "" {
				t.Fatalf("unavailable %s carries a version", tool.Name)
			}
		}
		slices.Sort(names)
		if !slices.Equal(names, []string{"tshark"}) {
			t.Fatal(names)
		}
	})
	t.Run("rejects_scope_before_execution", func(t *testing.T) {
		runner := newRunner(t)
		_, err := runner.InspectServices(deadline(t), "0.0.0.0/0", []int{80})
		requireCode(t, err, "invalid_input")
	})
	t.Run("loopback_inspection_or_honest_state", func(t *testing.T) {
		result, err := newRunner(t).InspectServices(deadline(t), "127.0.0.1/32", []int{1})
		RequireHonestState(t, err, "unsupported")
		if err != nil || result.Scanner != "native" {
			return
		}
		prefix := netip.MustParsePrefix("127.0.0.1/32")
		if len(result.Hosts) != 1 || len(result.Hosts[0].Ports) != 1 || result.Hosts[0].Ports[0].Port != 1 || result.Hosts[0].Ports[0].Protocol != "tcp" {
			t.Fatalf("%+v", result)
		}
		ip, parseErr := netip.ParseAddr(result.Hosts[0].Addresses[0].Address)
		if parseErr != nil || !prefix.Contains(ip) {
			t.Fatalf("address %v", result.Hosts[0].Addresses)
		}
	})
}

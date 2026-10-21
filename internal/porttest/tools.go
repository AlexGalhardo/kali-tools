package porttest

import (
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
		if !slices.Equal(names, []string{"nmap", "tshark"}) {
			t.Fatal(names)
		}
	})
	t.Run("rejects_scope_before_execution", func(t *testing.T) {
		runner := newRunner(t)
		_, err := runner.RunNmap(deadline(t), "0.0.0.0/0", []int{80})
		requireCode(t, err, "invalid_input")
	})
	t.Run("unavailable_tool_is_a_typed_state", func(t *testing.T) {
		runner := newRunner(t)
		ctx := deadline(t)
		for _, tool := range runner.Inventory(ctx) {
			if tool.Name == "nmap" {
				if tool.State == "available" {
					t.Log("available Nmap; unavailable-state assertion does not apply")
					return
				}
				_, err := runner.RunNmap(ctx, "127.0.0.1/32", []int{80})
				requireCode(t, err, tool.State)
				t.Logf("honest-state branch: %s", tool.State)
				return
			}
		}
		t.Fatal("missing Nmap inventory")
	})
}

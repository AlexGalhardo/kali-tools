//go:build windows

package platform

import "testing"

func TestNativeInventory(t *testing.T) {
	data, e := Inventory()
	if e != nil {
		t.Fatal(e)
	}
	if len(data.Adapters) == 0 || len(data.Counters) == 0 {
		t.Fatal("expected host adapters and counters")
	}
	seen := map[string]bool{}
	for _, c := range data.Counters {
		if seen[c.ID] {
			t.Fatal("duplicate counter identity")
		}
		seen[c.ID] = true
	}
}

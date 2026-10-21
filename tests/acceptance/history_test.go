package acceptance

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func HistorySpec(t *testing.T, newDriver scenarioDriver) {
	t.Helper()
	probe := newDriver(t)
	if _, err := probe.History(testContext(t)); errors.Is(err, errUnsupported) {
		t.Skip("history: CLI command arrives in C01")
	}
	t.Run("discovery_summary_without_target", func(t *testing.T) {
		d := newDriver(t)
		if _, err := d.Discover(testContext(t), "127.0.0.1/32", []int{openPort(t)}, 500*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		rows, err := d.History(testContext(t))
		if err != nil || len(rows) != 1 || rows[0].Kind != "discovery" || rows[0].State != "completed" || rows[0].Summary != "Discovery completed: 1 endpoints checked, 1 open" {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
		raw, err := json.Marshal(rows)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "127.0.0.1") {
			t.Fatal("target persisted")
		}
	})
	t.Run("clear_removes_terminal_jobs", func(t *testing.T) {
		d := newDriver(t)
		ctx := testContext(t)
		if _, err := d.Discover(ctx, "127.0.0.1/32", []int{openPort(t)}, 500*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if err := d.ClearHistory(ctx); err != nil {
			t.Fatal(err)
		}
		rows, err := d.History(ctx)
		if err != nil || len(rows) != 0 {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
	})
}

package acceptance

import (
	"fmt"
	"testing"
	"time"
)

func HistorySpec(t *testing.T, newDriver scenarioDriver) {
	t.Helper()
	t.Run("discovery_summary_records_exact_scope", func(t *testing.T) {
		d := newDriver(t)
		port := openPort(t)
		if _, err := d.Discover(testContext(t), "127.0.0.1/32", []int{port}, 500*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		rows, err := d.History(testContext(t))
		if err != nil || len(rows) != 1 || rows[0].Kind != "discovery" || rows[0].State != "completed" || rows[0].Summary != fmt.Sprintf("Discovery completed: 127.0.0.1/32 ports %d: 1 endpoints checked, 1 open", port) {
			t.Fatalf("rows=%v err=%v", rows, err)
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

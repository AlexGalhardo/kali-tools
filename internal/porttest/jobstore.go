package porttest

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

func JobStoreContract(t *testing.T, newStore func(*testing.T) app.JobStore) {
	t.Helper()
	t.Run("put_then_history", func(t *testing.T) {
		s := newStore(t)
		ctx := deadline(t)
		created := time.Date(2026, 10, 21, 1, 2, 3, 0, time.FixedZone("offset", 3600))
		finished := created.Add(time.Second)
		for index, state := range []job.State{job.Queued, job.Running, job.Completed} {
			status := job.Status{ID: "one", Kind: "tcp", State: state, Created: created.Add(time.Duration(index) * time.Hour)}
			if state.Terminal() {
				status.Finished = &finished
			}
			if err := s.PutJob(ctx, status); err != nil {
				t.Fatal(err)
			}
		}
		rows, err := s.History(ctx)
		if err != nil || len(rows) != 1 {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
		got := rows[0]
		if got.State != job.Completed || got.Finished == nil || got.Finished.Location() != time.UTC || !got.Finished.Equal(finished) || !got.Created.Equal(created) || got.Created.Location() != time.UTC {
			t.Fatalf("status=%+v", got)
		}
	})
	t.Run("history_newest_first_and_bounded", func(t *testing.T) {
		for _, count := range []int{HistoryLimit, HistoryLimit + 1} {
			t.Run(fmt.Sprint(count), func(t *testing.T) {
				s := newStore(t)
				// 201 separate SQLite commits miss the 3s contract deadline under the Windows race detector.
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				t.Cleanup(cancel)
				base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				for i := range count {
					created := base.Add(time.Duration(i) * time.Second)
					if err := s.PutJob(ctx, job.Status{ID: job.ID(fmt.Sprint(i)), Kind: "tcp", State: job.Completed, Created: created, Finished: &created}); err != nil {
						t.Fatal(err)
					}
				}
				rows, err := s.History(ctx)
				if err != nil || len(rows) != HistoryLimit {
					t.Fatalf("rows=%d err=%v", len(rows), err)
				}
				for i, row := range rows {
					if row.ID != job.ID(fmt.Sprint(count-1-i)) {
						t.Fatalf("index %d id=%s", i, row.ID)
					}
				}
			})
		}
	})
	t.Run("clear_keeps_active", func(t *testing.T) {
		s := newStore(t)
		ctx := deadline(t)
		for _, state := range []job.State{job.Queued, job.Running, job.Completed, job.Failed, job.Cancelled} {
			if err := s.PutJob(ctx, job.Status{ID: job.ID(state), Kind: "tcp", State: state, Created: time.Now().UTC()}); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Clear(ctx); err != nil {
			t.Fatal(err)
		}
		rows, err := s.History(ctx)
		if err != nil || len(rows) != 2 {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
		for _, row := range rows {
			if row.State != job.Queued && row.State != job.Running {
				t.Fatal(row)
			}
		}
	})
	t.Run("retain_drops_old_terminal_jobs", func(t *testing.T) {
		s := newStore(t)
		ctx := deadline(t)
		cut := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
		for _, state := range []job.State{job.Queued, job.Running, job.Completed, job.Failed, job.Cancelled} {
			for _, offset := range []time.Duration{-time.Hour, time.Hour} {
				if err := s.PutJob(ctx, job.Status{ID: job.ID(fmt.Sprintf("%s/%d", state, offset)), Kind: "tcp", State: state, Created: cut.Add(offset)}); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := s.Retain(ctx, cut); err != nil {
			t.Fatal(err)
		}
		rows, err := s.History(ctx)
		if err != nil || len(rows) != 7 {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
		for _, row := range rows {
			if row.State.Terminal() && row.Created.Before(cut) {
				t.Fatal(row)
			}
		}
	})
	t.Run("cancelled_context_writes_nothing", func(t *testing.T) {
		s := newStore(t)
		ctx, cancel := context.WithCancel(deadline(t))
		cancel()
		if err := s.PutJob(ctx, job.Status{ID: "cancelled", Kind: "tcp", State: job.Queued, Created: time.Now().UTC()}); err == nil {
			t.Fatal("cancelled write succeeded")
		}
		rows, err := s.History(deadline(t))
		if err != nil || len(rows) != 0 {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
	})
	t.Run("status_has_no_target_fields", func(t *testing.T) {
		var fields []string
		for field := range reflect.TypeFor[job.Status]().Fields() {
			fields = append(fields, field.Name)
		}
		if !slices.Equal(fields, []string{"ID", "Kind", "State", "Created", "Finished", "Summary"}) {
			t.Fatal(fields)
		}
	})
	t.Run("round_trip_is_verbatim", func(t *testing.T) {
		s := newStore(t)
		ctx := deadline(t)
		status := job.Status{ID: "verbatim", Kind: "tcp", State: job.Completed, Created: time.Now().UTC(), Summary: "synthetic-psk"}
		if err := s.PutJob(ctx, status); err != nil {
			t.Fatal(err)
		}
		rows, err := s.History(ctx)
		if err != nil || len(rows) != 1 || rows[0].Summary != status.Summary {
			t.Fatalf("rows=%v err=%v", rows, err)
		}
	})
}

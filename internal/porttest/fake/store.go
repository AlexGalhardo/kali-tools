package fake

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/porttest"
)

func NewStore() *Store {
	return &Store{statuses: make(map[job.ID]job.Status), settings: app.Settings{Language: "en", RetentionDays: 30}}
}

type Store struct {
	mu       sync.Mutex
	statuses map[job.ID]job.Status
	settings app.Settings
}

func (s *Store) PutJob(ctx context.Context, st job.Status) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fake store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.statuses == nil {
		s.statuses = make(map[job.ID]job.Status)
	}
	if previous, ok := s.statuses[st.ID]; ok {
		st.Created = previous.Created
		st.Kind = previous.Kind
	}
	st.Created = st.Created.UTC()
	if st.Finished != nil {
		finished := st.Finished.UTC()
		st.Finished = &finished
	}
	s.statuses[st.ID] = cloneStatus(st)
	return nil
}

func (s *Store) History(ctx context.Context) ([]job.Status, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("fake store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]job.Status, 0, len(s.statuses))
	for _, st := range s.statuses {
		rows = append(rows, cloneStatus(st))
	}
	slices.SortFunc(rows, func(a, b job.Status) int { return b.Created.Compare(a.Created) })
	return rows[:min(len(rows), porttest.HistoryLimit)], nil
}

func (s *Store) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fake store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, st := range s.statuses {
		if st.State.Terminal() {
			delete(s.statuses, id)
		}
	}
	return nil
}

func (s *Store) Retain(ctx context.Context, before time.Time) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fake store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, st := range s.statuses {
		if st.State.Terminal() && st.Created.Before(before) {
			delete(s.statuses, id)
		}
	}
	return nil
}

func cloneStatus(st job.Status) job.Status {
	if st.Finished != nil {
		finished := *st.Finished
		st.Finished = &finished
	}
	return st
}

func (s *Store) Settings(ctx context.Context) (app.Settings, error) {
	if err := ctx.Err(); err != nil {
		return app.Settings{}, fmt.Errorf("fake settings: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settings.Language == "" {
		return app.Settings{Language: "en", RetentionDays: 30}, nil
	}
	return s.settings, nil
}
func (s *Store) SaveSettings(ctx context.Context, value app.Settings) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("fake settings: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if (value.Language != "en" && value.Language != "pt-BR") || value.RetentionDays < porttest.MinRetentionDays || value.RetentionDays > porttest.MaxRetentionDays {
		return domain.Invalid("invalid settings")
	}
	s.settings = value
	return nil
}

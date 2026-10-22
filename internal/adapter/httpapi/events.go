package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const (
	sseKeepAlive    = 15 * time.Second
	sseWriteTimeout = 10 * time.Second
	sseRetryMS      = 3000
)

func (s *Server) handleEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case s.subscribers <- struct{}{}:
			defer func() { <-s.subscribers }()
		default:
			writeError(w, r, requestError(http.StatusTooManyRequests, domain.CodeConflict, "too many event streams"))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		rc := http.NewResponseController(w)
		write := func(value string) error {
			if err := rc.SetWriteDeadline(time.Now().Add(sseWriteTimeout)); err != nil {
				return err
			}
			if _, err := fmt.Fprint(w, value); err != nil {
				return err
			}
			return rc.Flush()
		}
		events, dropped := s.app.Subscribe(r.Context())
		s.app.StartMetrics(r.Context())
		if err := write(fmt.Sprintf("retry: %d\n\n", sseRetryMS)); err != nil {
			return
		}
		if last, err := strconv.ParseUint(r.Header.Get("Last-Event-ID"), 10, 64); err == nil {
			if write(fmt.Sprintf("event: resync\ndata: {\"reason\":\"reconnect\",\"last_seq\":%d}\n\n", last)) != nil {
				return
			}
		}
		ticker := time.NewTicker(s.keepAlive)
		defer ticker.Stop()
		var lastDropped uint64
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				if write(": keep-alive\n\n") != nil {
					return
				}
			case event, ok := <-events:
				if !ok {
					return
				}
				if count := dropped(); count > lastDropped {
					if write(fmt.Sprintf("event: resync\ndata: {\"reason\":\"dropped\",\"dropped\":%d}\n\n", count)) != nil {
						return
					}
					lastDropped = count
				}
				topic, value, ok := contract.EventPayload(event)
				if !ok {
					continue
				}
				raw, err := json.Marshal(value)
				if err != nil {
					return
				}
				if write(fmt.Sprintf("id: %d\nevent: %s\ndata: %s\n\n", event.Seq, topic, raw)) != nil {
					return
				}
			}
		}
	}
}

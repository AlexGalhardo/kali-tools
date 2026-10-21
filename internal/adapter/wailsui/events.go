package wailsui

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

func (b *Bindings) forward(ctx, fctx context.Context, events <-chan job.Event) {
	defer close(b.done)
	for {
		select {
		case <-fctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			name, payload, ok := contract.EventPayload(event)
			if ok {
				b.emit(ctx, name, payload)
			}
		}
	}
}

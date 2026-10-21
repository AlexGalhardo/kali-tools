package platform

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/app"
)

type Adapter = app.HostAdapter
type Snapshot = app.HostSnapshot
type Host struct{}

func (Host) Inventory(ctx context.Context) (app.HostSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return app.HostSnapshot{}, err
	}
	return Inventory()
}

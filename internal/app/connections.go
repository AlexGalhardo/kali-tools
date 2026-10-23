package app

import (
	"context"

	"github.com/AlexGalhardo/kali-tools/internal/conn"
)

func (a *Application) Connections(ctx context.Context) (conn.Table, error) {
	if err := ctx.Err(); err != nil {
		return conn.Table{Connections: []conn.Connection{}}, err
	}
	return conn.Read(ctx)
}

func (a *Application) Throughput(ctx context.Context, req conn.Request) (conn.Sample, error) {
	if err := ctx.Err(); err != nil {
		return conn.Sample{}, err
	}
	return conn.Measure(ctx, req)
}

package app

import (
	"context"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/conn"
)

func (a *Application) Connections(ctx context.Context) (conn.Table, error) {
	if err := ctx.Err(); err != nil {
		return conn.Table{Connections: []conn.Connection{}}, err
	}
	return conn.Read(ctx)
}

func (a *Application) Throughput(ctx context.Context, duration time.Duration, iperfJSON []byte) (conn.Sample, error) {
	if err := ctx.Err(); err != nil {
		return conn.Sample{}, err
	}
	return conn.Measure(ctx, conn.Request{Duration: duration, IperfJSON: iperfJSON})
}

package conn

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// Measure runs a loopback exchange or parses an iperf3 -J document.
func Measure(ctx context.Context, req Request) (Sample, error) {
	if len(req.IperfJSON) > 0 && req.Duration != 0 {
		return Sample{}, domain.Invalid("throughput takes a duration or an iperf report, not both")
	}
	if len(req.IperfJSON) > 0 {
		return ParseIperfJSON(req.IperfJSON)
	}
	if req.Duration == 0 {
		return Sample{}, domain.Invalid("throughput requires a duration or an iperf report")
	}
	return Exchange(ctx, req.Duration)
}

// Exchange sends bytes to a listener on 127.0.0.1 until duration elapses or ctx is cancelled.
// The sample names that endpoint and duration. It does not estimate bandwidth.
func Exchange(ctx context.Context, duration time.Duration) (Sample, error) {
	if duration < minDuration || duration > maxDuration {
		return Sample{}, domain.Invalid("throughput duration must be 1ms..10s")
	}
	if err := ctx.Err(); err != nil {
		return Sample{}, err
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		if ctx.Err() != nil {
			return Sample{}, ctx.Err()
		}
		return Sample{}, err
	}
	defer ln.Close()
	endpoint := ln.Addr().String()
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil || host != "127.0.0.1" {
		return Sample{}, domain.Invalid("throughput endpoint must be loopback")
	}

	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- conn
	}()

	client, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		if ctx.Err() != nil {
			return Sample{}, ctx.Err()
		}
		return Sample{}, err
	}
	defer client.Close()

	var server net.Conn
	select {
	case server = <-accepted:
	case err := <-acceptErr:
		if ctx.Err() != nil {
			return Sample{}, ctx.Err()
		}
		return Sample{}, err
	case <-ctx.Done():
		return Sample{}, ctx.Err()
	}
	defer server.Close()

	doneWatch := make(chan struct{})
	defer close(doneWatch)
	go func() {
		select {
		case <-ctx.Done():
			_ = client.Close()
			_ = server.Close()
		case <-doneWatch:
		}
	}()

	countCh := make(chan uint64, 1)
	go func() {
		buf := make([]byte, 4096)
		var n uint64
		for {
			read, err := server.Read(buf)
			n += uint64(read)
			if err != nil {
				countCh <- n
				return
			}
		}
	}()

	payload := make([]byte, 4096)
	if err := client.SetWriteDeadline(time.Now().Add(duration)); err != nil {
		return Sample{}, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return Sample{}, err
		}
		_, err := client.Write(payload)
		if err == nil {
			continue
		}
		if ctx.Err() != nil {
			return Sample{}, ctx.Err()
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			break
		}
		return Sample{}, err
	}
	if err := client.Close(); err != nil {
		return Sample{}, err
	}
	select {
	case n := <-countCh:
		return Sample{Endpoint: endpoint, DurationMS: duration.Milliseconds(), ByteCount: n, Source: "kali-tools"}, nil
	case <-ctx.Done():
		return Sample{}, ctx.Err()
	}
}

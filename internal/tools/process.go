package tools

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

type commandOutput struct {
	buffer   bytes.Buffer
	limit    int
	cancel   context.CancelFunc
	overflow bool
}

func (b *commandOutput) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		b.overflow = true
		b.cancel()
		return 0, errors.New("tool output limit exceeded")
	}
	return b.buffer.Write(p)
}

func runCommand(parent context.Context, path string, args []string, limit int) ([]byte, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.WaitDelay = time.Second
	out := &commandOutput{limit: limit, cancel: cancel}
	stderr := &commandOutput{limit: 8192, cancel: cancel}
	cmd.Stdout = out
	cmd.Stderr = stderr
	started, cleanup, err := prepareCommand(cmd)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	if err = started(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if parent.Err() != nil {
			return nil, parent.Err()
		}
		return nil, err
	}
	if ctx.Err() != nil {
		_ = cmd.Cancel()
	}
	err = cmd.Wait()
	if out.overflow || stderr.overflow {
		return out.buffer.Bytes(), errors.New("tool output limit exceeded")
	}
	if parent.Err() != nil {
		return out.buffer.Bytes(), parent.Err()
	}
	// Tool stderr may contain probed content; do not promote it into ordinary errors.
	return out.buffer.Bytes(), err
}

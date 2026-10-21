package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
)

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	stopReset := context.AfterFunc(ctx, stop)
	defer stopReset()
	if getenv("NO_COLOR") != "" {
		args = append([]string{"--no-color"}, args...)
	}
	return runWithBootstrap(ctx, args, stdout, stderr, bootstrap.New)
}

func runWithBootstrap(ctx context.Context, args []string, stdout, stderr io.Writer, boot func(context.Context, bootstrap.Config) (*app.Application, func() error, error)) int {
	if !cli.NeedsApp(args) {
		return cli.Run(ctx, nil, args, stdout, stderr)
	}
	a, cleanup, err := boot(ctx, bootstrap.Config{})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() {
		if err := cleanup(); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
		}
	}()
	return cli.Run(ctx, a, args, stdout, stderr)
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

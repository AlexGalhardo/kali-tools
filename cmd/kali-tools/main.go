package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/cli"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/bootstrap"
)

const cliDataPath = ":memory:"

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	return runWithBootstrap(ctx, args, stdout, stderr, bootstrap.New)
}

func runWithBootstrap(ctx context.Context, args []string, stdout, stderr io.Writer, boot func(context.Context, bootstrap.Config) (*app.Application, func() error, error)) int {
	a, cleanup, err := boot(ctx, bootstrap.Config{DataPath: cliDataPath})
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	cancel()
	os.Exit(code)
}

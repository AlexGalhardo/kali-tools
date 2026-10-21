package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/job"
)

func AwaitCaptureForTest(ctx context.Context, events <-chan job.Event, stop func(), limit time.Duration) (capture.LiveResult, error) {
	return awaitCapture(ctx, events, stop, limit)
}
func ParseCodedErrorForTest(message string) error { return parseCodedError(message) }

func CommandPathsForTest() []string {
	paths := []string{"help"}
	for _, command := range commands {
		paths = append(paths, command.path)
	}
	return paths
}

func TestProgressLineBound(t *testing.T) {
	var out bytes.Buffer
	p := progressPrinter{out: &out, lang: "pt-BR"}
	for range maxProgressLines {
		p.print(job.Status{ID: "0123456789abcdef", Kind: "tcp", State: job.Running})
	}
	if strings.Count(out.String(), "\n") != maxProgressLines || !strings.Contains(out.String(), "job 01234567 tcp: executando") {
		t.Fatal("progress at limit")
	}
	before := out.String()
	p.print(job.Status{ID: "0123456789abcdef", Kind: "tcp", State: job.Completed})
	if out.String() != before {
		t.Fatal("progress exceeded limit")
	}
}

func TestPortListLimits(t *testing.T) {
	value := strings.Repeat("65535,", 31) + "65535"
	if len(value) != maxPortsArg {
		t.Fatal("invalid boundary fixture")
	}
	if ports, err := parsePorts(value); err != nil || len(ports) != 32 {
		t.Fatal(ports, err)
	}
	for _, value := range []string{value + " ", strings.Repeat("1,", 32) + "1"} {
		if _, err := parsePorts(value); err == nil {
			t.Fatal("oversize port list accepted")
		}
	}
}

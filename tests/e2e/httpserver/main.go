//go:build e2e

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/httpapi"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/wifi"
)

func makeFixture(ctx context.Context, path string) error {
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: net.IPv4(192, 0, 2, 1), DstIP: net.IPv4(192, 0, 2, 2), Protocol: layers.IPProtocolUDP}
	udp := &layers.UDP{SrcPort: 12345, DstPort: 53}
	dns := &layers.DNS{ID: 1, RD: true, Questions: []layers.DNSQuestion{{Name: []byte("example.test"), Type: layers.DNSTypeA, Class: layers.DNSClassIN}}}
	data := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(data, gopacket.SerializeOptions{FixLengths: true}, ip, udp, dns); err != nil {
		return err
	}
	artifact := capture.NewArtifact(layers.LinkTypeRaw)
	if err := artifact.Add(data.Bytes(), gopacket.CaptureInfo{Timestamp: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC), CaptureLength: len(data.Bytes()), Length: len(data.Bytes())}); err != nil {
		return err
	}
	return artifact.Save(ctx, path, "pcap")
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("http fixture", flag.ContinueOnError)
	assets := flags.String("assets", "frontend/dist", "built frontend directory")
	listen := flags.String("listen", "127.0.0.1:4174", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "kali-tools-e2e-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	capturePath := filepath.Join(dir, "fixture.pcap")
	if err := makeFixture(ctx, capturePath); err != nil {
		return err
	}
	tcp, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			connection, err := tcp.Accept()
			if err != nil {
				return
			}
			_ = connection.Close()
		}
	}()
	defer func() { _ = tcp.Close(); <-stopped }()
	store := NewStore()
	a := app.New(app.Deps{Jobs: store, Settings: store,
		Host:        Host{Snapshot: app.HostSnapshot{Adapters: []app.HostAdapter{{Index: 1, Name: "Fixture Ethernet", Gateways: []string{"192.0.2.1"}, DNS: []string{"192.0.2.53"}}}, Counters: []network.Counter{{ID: "1", Received: 200, Sent: 100}}}},
		WiFi:        WiFi{Network: wifi.Network{SSID: "Fixture Network", Profile: "Fixture", BSSID: "02:00:00:00:00:02", Signal: 80, Authentication: 7, State: "connected"}, Secret: wifi.Secret{State: "available", Value: "synthetic-test-key"}},
		Tools:       Tools{Tools: []app.Tool{{Name: "nmap", State: "dependency_missing"}, {Name: "tshark", State: "dependency_missing"}}},
		Diagnostics: network.Diagnostics{Prober: Prober{Values: []string{"echo reply"}}},
		Capture: LiveCapture{CaptureFunc: func(ctx context.Context, _ capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
			result, artifact, err := capture.ReadArtifactFile(capturePath)
			return capture.LiveResult{Packets: result.Packets, Provider: "fixture"}, artifact, err
		}},
	})
	defer func() { _ = a.Close() }()
	server, err := httpapi.New(a, httpapi.Options{Addr: *listen, Assets: os.DirFS(*assets)})
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "KT_URL=%s\nKT_FIXTURE_TCP=%s\nKT_FIXTURE_PCAP=%s\nKT_READY\n", server.URL(), tcp.Addr(), capturePath); err != nil {
		return err
	}
	return server.Run(ctx)
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

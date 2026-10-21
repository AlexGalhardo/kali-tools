package tools

import (
	"context"
	"encoding/xml"
	"os/exec"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

type Tool = app.Tool

func inspect(ctx context.Context, name string) Tool {
	item := Tool{Name: name, State: "dependency_missing"}
	path, err := exec.LookPath(name)
	if err != nil {
		return item
	}
	item.Path = path
	probe, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	raw, err := runCommand(probe, path, []string{"--version"}, 8192)
	if err != nil {
		item.State = "failed"
		return item
	}
	item.Version = strings.TrimSpace(strings.SplitN(string(raw), "\n", 2)[0])
	item.State = "available"
	if !supportedVersion(name, item.Version) {
		item.State = "unsupported"
	}
	return item
}

func Inventory(ctx context.Context) []Tool {
	return []Tool{inspect(ctx, "nmap"), inspect(ctx, "tshark")}
}

type Nmap = app.Nmap
type NmapFinished = app.NmapFinished
type NmapState = app.NmapState
type NmapAddress = app.NmapAddress
type NmapService = app.NmapService
type NmapPort = app.NmapPort
type NmapHost = app.NmapHost

// Runner adapts optional executable operations to the application port.
type Runner struct{}

func (Runner) Inventory(ctx context.Context) []app.Tool { return Inventory(ctx) }
func (Runner) RunNmap(ctx context.Context, cidr string, ports []int) (app.Nmap, error) {
	return RunNmap(ctx, cidr, ports)
}
func ParseNmap(raw []byte) (Nmap, error) {
	if len(raw) > 4<<20 {
		return Nmap{}, domain.Invalid("Nmap output exceeds 4 MiB")
	}
	var result Nmap
	e := xml.Unmarshal(raw, &result)
	if e != nil {
		return Nmap{}, domain.Invalid("invalid Nmap XML")
	}
	if len(result.Hosts) > 256 {
		return Nmap{}, domain.Invalid("Nmap host count exceeds limit")
	}
	for _, host := range result.Hosts {
		if len(host.Ports) > 32 || len(host.Addresses) > 32 {
			return Nmap{}, domain.Invalid("Nmap result count exceeds limit")
		}
		if len(host.Status.State) > 32 {
			return Nmap{}, domain.Invalid("invalid Nmap host state")
		}
		for _, a := range host.Addresses {
			if len(a.Address) > 64 || len(a.Type) > 16 {
				return Nmap{}, domain.Invalid("invalid Nmap address")
			}
		}
		for _, p := range host.Ports {
			if p.Port < 1 || p.Port > 65535 || (p.Protocol != "tcp" && p.Protocol != "udp") || len(p.Service.Name) > 256 || len(p.Service.Product) > 1024 || len(p.Service.Version) > 256 || (p.Service.Method != "" && p.Service.Method != "table" && p.Service.Method != "probed") {
				return Nmap{}, domain.Invalid("invalid Nmap service data")
			}
		}
	}
	return result, nil
}

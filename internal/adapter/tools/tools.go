package tools

import (
	"context"
	"encoding/xml"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"os/exec"
	"strings"
	"time"
)

type Tool struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

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

type Nmap struct {
	XMLName  xml.Name     `xml:"nmaprun" json:"-"`
	Scanner  string       `xml:"scanner,attr" json:"scanner"`
	Version  string       `xml:"version,attr" json:"version"`
	Finished NmapFinished `xml:"runstats>finished" json:"finished"`
	Hosts    []NmapHost   `xml:"host" json:"hosts"`
}
type NmapFinished struct {
	Exit string `xml:"exit,attr" json:"exit"`
}
type NmapState struct {
	State string `xml:"state,attr" json:"state"`
}
type NmapAddress struct {
	Address string `xml:"addr,attr" json:"address"`
	Type    string `xml:"addrtype,attr" json:"type"`
}
type NmapService struct {
	Name    string `xml:"name,attr" json:"name"`
	Product string `xml:"product,attr" json:"product"`
	Version string `xml:"version,attr" json:"version"`
	Method  string `xml:"method,attr" json:"method"`
}
type NmapPort struct {
	Service  NmapService `xml:"service" json:"service"`
	Port     int         `xml:"portid,attr" json:"port"`
	Protocol string      `xml:"protocol,attr" json:"protocol"`
	State    NmapState   `xml:"state" json:"state"`
}
type NmapHost struct {
	Status    NmapState     `xml:"status" json:"status"`
	Addresses []NmapAddress `xml:"address" json:"addresses"`
	Ports     []NmapPort    `xml:"ports>port" json:"ports"`
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

package tools

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"github.com/AlexGalhardo/hacker-tools/internal/domain"
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
type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, fmt.Errorf("tool output exceeds %d bytes", b.limit)
	}
	return b.Buffer.Write(p)
}
func Inventory(ctx context.Context) []Tool {
	out := []Tool{}
	for _, name := range []string{"nmap", "tshark"} {
		item := Tool{Name: name, State: "dependency_missing"}
		path, e := exec.LookPath(name)
		if e == nil {
			item.Path = path
			probe, cancel := context.WithTimeout(ctx, 3*time.Second)
			cmd := exec.CommandContext(probe, path, "--version")
			cmd.WaitDelay = time.Second
			buf := &limitedBuffer{limit: 8192}
			cmd.Stdout = buf
			cmd.Stderr = buf
			e = cmd.Run()
			cancel()
			item.State = "available"
			if e != nil {
				item.State = "failed"
			} else {
				item.Version = strings.SplitN(buf.String(), "\n", 2)[0]
			}
		}
		out = append(out, item)
	}
	return out
}

type Nmap struct {
	Hosts []NmapHost `xml:"host" json:"hosts"`
}
type NmapHost struct {
	Status struct {
		State string `xml:"state,attr" json:"state"`
	} `xml:"status" json:"status"`
	Addresses []struct {
		Address string `xml:"addr,attr" json:"address"`
		Type    string `xml:"addrtype,attr" json:"type"`
	} `xml:"address" json:"addresses"`
	Ports []struct {
		Port     int    `xml:"portid,attr" json:"port"`
		Protocol string `xml:"protocol,attr" json:"protocol"`
		State    struct {
			State string `xml:"state,attr" json:"state"`
		} `xml:"state" json:"state"`
	} `xml:"ports>port" json:"ports"`
}

func ParseNmap(raw []byte) (Nmap, error) {
	if len(raw) > 4<<20 {
		return Nmap{}, domain.Invalid("Nmap output exceeds 4 MiB")
	}
	var result Nmap
	e := xml.Unmarshal(raw, &result)
	if e != nil {
		return result, domain.Invalid("invalid Nmap XML")
	}
	return result, nil
}

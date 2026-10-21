//go:build linux

package platform

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/kali-tools/internal/network"
)

func Inventory() (Snapshot, error) {
	out := Snapshot{Adapters: []Adapter{}, Counters: []network.Counter{}}
	interfaces, e := net.Interfaces()
	if e != nil {
		return out, e
	}
	dns := []string{}
	if raw, e := os.ReadFile("/etc/resolv.conf"); e == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			f := strings.Fields(line)
			if len(f) > 1 && f[0] == "nameserver" {
				dns = append(dns, f[1])
			}
		}
	}
	gateways := map[string][]string{}
	if raw, e := os.ReadFile("/proc/net/route"); e == nil {
		for _, line := range strings.Split(string(raw), "\n") {
			f := strings.Fields(line)
			if len(f) > 3 && f[1] == "00000000" {
				b, e := hex.DecodeString(f[2])
				if e == nil && len(b) == 4 {
					gateways[f[0]] = append(gateways[f[0]], net.IPv4(b[3], b[2], b[1], b[0]).String())
				}
			}
		}
	}
	for _, i := range interfaces {
		gw := gateways[i.Name]
		if gw == nil {
			gw = []string{}
		}
		out.Adapters = append(out.Adapters, Adapter{Index: uint32(i.Index), Name: i.Name, Gateways: gw, DNS: append([]string{}, dns...)})
	}
	f, e := os.Open("/proc/net/dev")
	if e != nil {
		return out, e
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		name, data, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(data)
		if len(fields) < 16 {
			continue
		}
		rx, e := strconv.ParseUint(fields[0], 10, 64)
		if e != nil {
			return out, e
		}
		tx, e := strconv.ParseUint(fields[8], 10, 64)
		if e != nil {
			return out, e
		}
		out.Counters = append(out.Counters, network.Counter{ID: fmt.Sprint(strings.TrimSpace(name)), Received: rx, Sent: tx})
	}
	return out, scanner.Err()
}

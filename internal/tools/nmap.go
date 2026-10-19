package tools

import (
	"context"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

var nmapResultVersion = regexp.MustCompile(`^7\.[0-9]+(?:\.[0-9]+)?$`)
var nmapVersion = regexp.MustCompile(`^Nmap version (7)\.([0-9]+)(?:\.[0-9]+)?(?: |$)`)
var tsharkVersion = regexp.MustCompile(`^TShark \(Wireshark\) (4)\.([0-9]+)\.[0-9]+(?: |$)`)

func supportedVersion(name, line string) bool {
	pattern := nmapVersion
	minimum := 80
	if name == "tshark" {
		pattern = tsharkVersion
		minimum = 2
	} else if name != "nmap" {
		return false
	}
	match := pattern.FindStringSubmatch(line)
	if len(match) != 3 {
		return false
	}
	minor, err := strconv.Atoi(match[2])
	return err == nil && minor >= minimum
}

func nmapArguments(cidr string, ports []int) ([]string, error) {
	if err := network.ValidateDiscovery(cidr, ports, time.Second); err != nil {
		return nil, err
	}
	numbers := make([]string, len(ports))
	for i, p := range ports {
		numbers[i] = strconv.Itoa(p)
	}
	return []string{"-sT", "-sV", "--version-light", "-Pn", "-n", "--max-retries", "1", "--host-timeout", "15s", "--max-rate", "100", "-p", strings.Join(numbers, ","), "-oX", "-", cidr}, nil
}

// RunNmap uses its maintained service fingerprint database; native TCP discovery remains the default.
func RunNmap(parent context.Context, cidr string, ports []int) (Nmap, error) {
	args, err := nmapArguments(cidr, ports)
	if err != nil {
		return Nmap{}, err
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	tool := inspect(ctx, "nmap")
	if tool.State != "available" {
		return Nmap{}, &domain.Error{Code: tool.State, Message: "Nmap 7.80 or newer within major 7 must be installed separately"}
	}
	raw, err := runCommand(ctx, tool.Path, args, 4<<20)
	if err != nil {
		return Nmap{}, err
	}
	result, err := ParseNmap(raw)
	if err != nil {
		return Nmap{}, err
	}
	if result.Scanner != "nmap" || len(result.Version) > 32 || !nmapResultVersion.MatchString(result.Version) || !supportedVersion("nmap", "Nmap version "+result.Version) || result.Finished.Exit != "success" {
		return Nmap{}, domain.Invalid("incomplete or unsupported Nmap result")
	}
	scope, _ := netip.ParsePrefix(cidr)
	allowed := map[int]bool{}
	for _, port := range ports {
		allowed[port] = true
	}
	for _, host := range result.Hosts {
		found := false
		for _, address := range host.Addresses {
			if address.Type != "ipv4" {
				continue
			}
			ip, err := netip.ParseAddr(address.Address)
			if err != nil || !scope.Contains(ip) {
				return Nmap{}, domain.Invalid("Nmap result is outside the requested scope")
			}
			found = true
		}
		if !found {
			return Nmap{}, domain.Invalid("Nmap host has no scoped IPv4 address")
		}
		for _, port := range host.Ports {
			if port.Protocol != "tcp" || !allowed[port.Port] {
				return Nmap{}, domain.Invalid("Nmap result includes an unrequested port")
			}
		}
	}
	return result, nil
}

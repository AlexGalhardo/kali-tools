//go:build windows

package capture

import (
	"context"
	"errors"
	"github.com/AlexGalhardo/hacker-tools/internal/domain"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"golang.org/x/sys/windows"
	"net"
	"net/netip"
	"sync"
	"time"
	"unsafe"
)

func Live(ctx context.Context, r LiveRequest) (LiveResult, error) {
	result := LiveResult{Packets: []Packet{}, Provider: "Windows raw IPv4 socket", Started: time.Now().UTC()}
	ip, e := netip.ParseAddr(r.Address)
	if e != nil || !ip.Is4() || ip.IsLoopback() || r.DurationSeconds < 1 || r.DurationSeconds > 60 || r.PacketLimit < 1 || r.PacketLimit > MaxPackets {
		return result, domain.Invalid("select a local non-loopback IPv4, duration 1..60 and packet limit 1..10000")
	}
	addresses, e := net.InterfaceAddrs()
	if e != nil {
		return result, e
	}
	local := false
	for _, a := range addresses {
		p, e := netip.ParsePrefix(a.String())
		if e == nil && p.Addr() == ip {
			local = true
		}
	}
	if !local {
		return result, domain.Invalid("capture address must belong to a local interface")
	}
	var wsa windows.WSAData
	if e = windows.WSAStartup(0x202, &wsa); e != nil {
		return result, e
	}
	defer windows.WSACleanup()
	socket, e := windows.Socket(windows.AF_INET, windows.SOCK_RAW, windows.IPPROTO_IP)
	if e != nil {
		if errors.Is(e, windows.WSAEACCES) {
			return result, &domain.Error{Code: "permission_required", Message: "Raw IPv4 capture requires Windows administrator authorization"}
		}
		return result, e
	}
	var once sync.Once
	closeSocket := func() { once.Do(func() { windows.Closesocket(socket) }) }
	defer closeSocket()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(r.DurationSeconds)*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, closeSocket)
	defer stop()
	e = windows.Bind(socket, &windows.SockaddrInet4{Addr: ip.As4()})
	if e != nil {
		return result, e
	}
	mode := uint32(1)
	var bytes uint32
	e = windows.WSAIoctl(socket, 0x98000001, (*byte)(unsafe.Pointer(&mode)), 4, nil, 0, &bytes, nil, 0)
	if e != nil {
		return result, e
	}
	buf := make([]byte, 65536)
	for len(result.Packets) < r.PacketLimit {
		n, _, e := windows.Recvfrom(socket, buf, 0)
		if e != nil {
			if ctx.Err() != nil {
				break
			}
			return result, e
		}
		packet, e := Decode(buf[:n], layers.LinkTypeRaw, gopacket.CaptureInfo{Timestamp: time.Now().UTC(), CaptureLength: n, Length: n}, len(result.Packets)+1)
		if e != nil {
			return result, e
		}
		result.Packets = append(result.Packets, packet)
	}
	result.Finished = time.Now().UTC()
	if errors.Is(ctx.Err(), context.Canceled) {
		return result, ctx.Err()
	}
	return result, nil
}

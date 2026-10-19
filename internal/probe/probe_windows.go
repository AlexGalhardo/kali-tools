//go:build windows

package probe

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/AlexGalhardo/hacker-tools/internal/domain"
	"golang.org/x/sys/windows"
	"net"
	"time"
	"unsafe"
)

const Provider = "Windows ICMP API (IPv4)"

var dll = windows.NewLazySystemDLL("iphlpapi.dll")
var create = dll.NewProc("IcmpCreateFile")
var closeICMP = dll.NewProc("IcmpCloseHandle")
var send = dll.NewProc("IcmpSendEcho")

type options32 struct {
	TTL   byte
	TOS   byte
	Flags byte
	Size  byte
	Data  uint32
}

func Run(ctx context.Context, target string, trace bool) ([]string, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	addresses, e := net.DefaultResolver.LookupIP(ctx, "ip4", target)
	if e != nil {
		return nil, e
	}
	if len(addresses) == 0 {
		return nil, domain.Unsupported("Windows ICMP provider requires IPv4")
	}
	ip := addresses[0].To4()
	if ip == nil {
		return nil, domain.Unsupported("Windows ICMP provider requires IPv4")
	}
	handle, _, e := create.Call()
	if handle == ^uintptr(0) {
		if errors.Is(e, windows.ERROR_ACCESS_DENIED) {
			return nil, &domain.Error{Code: "permission_required", Message: "Windows denied ICMP handle creation"}
		}
		return nil, e
	}
	defer closeICMP.Call(handle)
	results := []string{}
	payload := []byte("hacker-tools")
	count := 4
	if trace {
		count = 20
	}
	received := 0
	for i := 1; i <= count; i++ {
		if e = ctx.Err(); e != nil {
			return results, e
		}
		timeout := 500 * time.Millisecond
		if deadline, ok := ctx.Deadline(); ok {
			timeout = min(timeout, time.Until(deadline))
		}
		if timeout < time.Millisecond {
			return results, context.DeadlineExceeded
		}
		ttl := byte(128)
		if trace {
			ttl = byte(i)
		}
		options := options32{TTL: ttl}
		reply := make([]byte, 512)
		n, _, callErr := send.Call(handle, uintptr(binary.LittleEndian.Uint32(ip)), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), uintptr(unsafe.Pointer(&options)), uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), uintptr(timeout.Milliseconds()))
		if n == 0 {
			results = append(results, fmt.Sprintf("probe=%d timeout_or_error=%v", i, callErr))
			continue
		}
		status := binary.LittleEndian.Uint32(reply[4:8])
		rtt := binary.LittleEndian.Uint32(reply[8:12])
		from := net.IP(reply[:4]).String()
		results = append(results, fmt.Sprintf("probe=%d address=%s status=%d rtt_ms=%d", i, from, status, rtt))
		if status == 0 {
			received++
			if trace {
				return results, nil
			}
		}
	}
	if !trace {
		results = append(results, fmt.Sprintf("sent=%d received=%d loss_percent=%d", count, received, (count-received)*100/count))
		if received == 0 {
			return results, fmt.Errorf("no ICMP echo replies")
		}
	} else {
		return results, fmt.Errorf("destination not reached within 20 hops")
	}
	return results, nil
}

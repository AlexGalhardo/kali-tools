//go:build windows

package platform

import (
	"errors"
	"fmt"
	"github.com/AlexGalhardo/hacker-tools/internal/network"
	"golang.org/x/sys/windows"
	"unsafe"
)

func Inventory() (Snapshot, error) {
	out := Snapshot{Adapters: []Adapter{}, Counters: []network.Counter{}}
	size := uint32(15000)
	var buf []byte
	for attempts := 0; attempts < 3; attempts++ {
		buf = make([]byte, size)
		e := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_GATEWAYS, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size)
		if errors.Is(e, windows.ERROR_BUFFER_OVERFLOW) {
			if size > 4<<20 {
				return out, fmt.Errorf("adapter buffer limit")
			}
			continue
		}
		if e != nil {
			return out, e
		}
		for a := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); a != nil; a = a.Next {
			item := Adapter{Index: a.IfIndex, Name: windows.UTF16PtrToString(a.FriendlyName), Gateways: []string{}, DNS: []string{}}
			for d := a.FirstDnsServerAddress; d != nil; d = d.Next {
				item.DNS = append(item.DNS, d.Address.IP().String())
			}
			for g := a.FirstGatewayAddress; g != nil; g = g.Next {
				item.Gateways = append(item.Gateways, g.Address.IP().String())
			}
			out.Adapters = append(out.Adapters, item)
		}
		break
	}
	var table *windows.MibIfTable2
	if e := windows.GetIfTable2Ex(0, &table); e != nil {
		return out, e
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	if table.NumEntries > 65536 {
		return out, fmt.Errorf("interface table exceeds limit")
	}
	for _, r := range unsafe.Slice(&table.Table[0], int(table.NumEntries)) {
		out.Counters = append(out.Counters, network.Counter{ID: fmt.Sprint(r.InterfaceLuid), Received: r.InOctets, Sent: r.OutOctets})
	}
	return out, nil
}

package platform

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const (
	maxNetlinkBytes     = 4 << 20
	maxProcARPBytes     = 1 << 20
	maxRawNeighborRows  = 65536
	maxNeighborMACBytes = 32
	nlmsgHeaderLen      = 16
	ndmsgLen            = 12
	rtattrHeaderLen     = 4
	rtmNewNeigh         = 28
	nlmsgDone           = 3
	nlmsgError          = 2
	ndaDst              = 1
	ndaLLAddr           = 2
	afInet              = 2
	afInet6             = 10
	nudIncomplete       = 0x01
	nudReachable        = 0x02
	nudStale            = 0x04
	nudDelay            = 0x08
	nudProbe            = 0x10
	nudFailed           = 0x20
	nudNoARP            = 0x40
	nudPermanent        = 0x80
	ntfRouter           = 0x80
)

func align4(size int) int { return (size + 3) &^ 3 }

func parseNetlinkNeighbors(raw []byte, names map[uint32]string) ([]network.Neighbor, error) {
	rows := []network.Neighbor{}
	if len(raw) > maxNetlinkBytes {
		return rows, domain.Invalid("neighbor netlink dump exceeds limit")
	}
	for len(raw) > 0 {
		if len(raw) < nlmsgHeaderLen {
			return nil, domain.Invalid("truncated neighbor netlink header")
		}
		size := uint64(binary.NativeEndian.Uint32(raw))
		if size < nlmsgHeaderLen || size > uint64(len(raw)) {
			return nil, domain.Invalid("invalid neighbor netlink message length")
		}
		kind := binary.NativeEndian.Uint16(raw[4:])
		if kind == nlmsgDone {
			return rows, nil
		}
		if kind == nlmsgError {
			return nil, &domain.Error{Code: domain.CodeFailed, Message: "neighbor netlink dump failed"}
		}
		if kind == rtmNewNeigh {
			row, ok, err := parseNetlinkNeighbor(raw[nlmsgHeaderLen:int(size)], names)
			if err != nil {
				return nil, err
			}
			if ok {
				rows = append(rows, row)
				if len(rows) == maxRawNeighborRows {
					return rows, nil
				}
			}
		}
		advance := align4(int(size))
		if advance > len(raw) {
			return nil, domain.Invalid("truncated neighbor netlink padding")
		}
		raw = raw[advance:]
	}
	return rows, nil
}

func parseNetlinkNeighbor(body []byte, names map[uint32]string) (network.Neighbor, bool, error) {
	if len(body) < ndmsgLen {
		return network.Neighbor{}, false, domain.Invalid("truncated neighbor netlink body")
	}
	family := body[0]
	index, state, flags := binary.NativeEndian.Uint32(body[4:]), binary.NativeEndian.Uint16(body[8:]), body[10]
	attrs := body[ndmsgLen:]
	var destination, mac []byte
	for len(attrs) > 0 {
		if len(attrs) < rtattrHeaderLen {
			return network.Neighbor{}, false, domain.Invalid("truncated neighbor netlink attribute")
		}
		size := int(binary.NativeEndian.Uint16(attrs))
		if size < rtattrHeaderLen || size > len(attrs) {
			return network.Neighbor{}, false, domain.Invalid("invalid neighbor netlink attribute length")
		}
		switch binary.NativeEndian.Uint16(attrs[2:]) {
		case ndaDst:
			destination = attrs[rtattrHeaderLen:size]
		case ndaLLAddr:
			mac = attrs[rtattrHeaderLen:size]
		}
		advance := align4(size)
		if advance > len(attrs) {
			return network.Neighbor{}, false, domain.Invalid("truncated neighbor attribute padding")
		}
		attrs = attrs[advance:]
	}
	if family != afInet && family != afInet6 {
		return network.Neighbor{}, false, nil
	}
	if (family == afInet && len(destination) != 4) || (family == afInet6 && len(destination) != 16) ||
		len(mac) > maxNeighborMACBytes {
		return network.Neighbor{}, false, domain.Invalid("invalid neighbor address length")
	}
	if state&nudNoARP != 0 {
		return network.Neighbor{}, false, nil
	}
	address, _ := netip.AddrFromSlice(destination)
	if address.IsLinkLocalUnicast() && address.Is6() && names[index] != "" {
		address = address.WithZone(names[index])
	}
	familyName := "ipv4"
	if family == afInet6 {
		familyName = "ipv6"
	}
	return network.Neighbor{
		Address: address.String(), MAC: neighborMAC(mac), Family: familyName,
		InterfaceIndex: index, InterfaceName: names[index],
		State: netlinkNeighborState(state), Router: flags&ntfRouter != 0,
	}, true, nil
}

func netlinkNeighborState(state uint16) string {
	for _, entry := range []struct {
		bit   uint16
		state string
	}{
		{nudPermanent, network.NeighborPermanent}, {nudReachable, network.NeighborReachable},
		{nudStale, network.NeighborStale}, {nudDelay, network.NeighborDelay},
		{nudProbe, network.NeighborProbe}, {nudIncomplete, network.NeighborIncomplete},
		{nudFailed, network.NeighborUnreachable},
	} {
		if state&entry.bit != 0 {
			return entry.state
		}
	}
	return network.NeighborUnknown
}

func neighborMAC(raw []byte) string {
	for _, value := range raw {
		if value != 0 {
			return net.HardwareAddr(raw).String()
		}
	}
	return ""
}

func parseProcNetARP(reader io.Reader, names map[string]uint32) ([]network.Neighbor, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxProcARPBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxProcARPBytes {
		return nil, domain.Invalid("neighbor procfs table exceeds limit")
	}
	rows := []network.Neighbor{}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), maxProcARPBytes+1)
	scanner.Scan() // Header.
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 6 {
			continue
		}
		address, err := netip.ParseAddr(fields[0])
		if err != nil || !address.Is4() {
			continue
		}
		flags, err := strconv.ParseUint(fields[2], 0, 32)
		if err != nil {
			continue
		}
		mac, err := net.ParseMAC(fields[3])
		if err != nil || len(mac) > maxNeighborMACBytes {
			continue
		}
		state := network.NeighborUnknown
		if flags&4 != 0 {
			state = network.NeighborPermanent
		} else if flags&2 == 0 {
			state = network.NeighborIncomplete
		}
		rows = append(rows, network.Neighbor{
			Address: address.String(), MAC: neighborMAC(mac), Family: "ipv4",
			InterfaceIndex: names[fields[5]], InterfaceName: fields[5], State: state,
		})
		if len(rows) == maxRawNeighborRows {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

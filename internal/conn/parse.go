package conn

import (
	"cmp"
	"encoding/hex"
	"net"
	"slices"
	"strconv"
	"strings"
)

type rawRow struct {
	protocol string
	local    string
	remote   string
	state    string
	inode    uint64
}

type owner struct {
	pid  int
	name string
}

var tcpStates = map[string]string{
	"01": "established",
	"02": "syn_sent",
	"03": "syn_recv",
	"04": "fin_wait1",
	"05": "fin_wait2",
	"06": "time_wait",
	"07": "close",
	"08": "close_wait",
	"09": "last_ack",
	"0A": "listen",
	"0B": "closing",
}

func parseNet(text, protocol string) []rawRow {
	var rows []rawRow
	for line := range strings.SplitSeq(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 || fields[0] == "sl" || !strings.Contains(fields[1], ":") {
			continue
		}
		local, ok := decodeEndpoint(fields[1])
		if !ok {
			continue
		}
		remote, ok := decodeEndpoint(fields[2])
		if !ok {
			continue
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil || inode == 0 {
			continue
		}
		state := strings.ToUpper(fields[3])
		name, known := tcpStates[state]
		if !known {
			name = strings.ToLower(state)
		}
		rows = append(rows, rawRow{protocol: protocol, local: local, remote: remote, state: name, inode: inode})
	}
	return rows
}

func decodeEndpoint(field string) (string, bool) {
	host, portHex, ok := strings.Cut(field, ":")
	if !ok {
		return "", false
	}
	port, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return "", false
	}
	ip, ok := decodeIP(host)
	if !ok {
		return "", false
	}
	return net.JoinHostPort(ip.String(), strconv.FormatUint(port, 10)), true
}

func decodeIP(value string) (net.IP, bool) {
	raw, err := hex.DecodeString(value)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return nil, false
	}
	for i := 0; i < len(raw); i += 4 {
		raw[i], raw[i+3] = raw[i+3], raw[i]
		raw[i+1], raw[i+2] = raw[i+2], raw[i+1]
	}
	return net.IP(raw), true
}

func assemble(raws []rawRow, owners map[uint64]owner, limited bool) Table {
	rows := make([]Connection, 0, len(raws))
	for _, row := range raws {
		who, ok := owners[row.inode]
		if !ok {
			continue
		}
		if len(rows) == maxRows {
			limited = true
			break
		}
		rows = append(rows, Connection{
			Protocol: row.protocol, Local: row.local, Remote: row.remote,
			State: row.state, PID: who.pid, Process: who.name,
		})
	}
	slices.SortFunc(rows, func(a, b Connection) int {
		return cmp.Or(
			cmp.Compare(a.Protocol, b.Protocol),
			cmp.Compare(a.Local, b.Local),
			cmp.Compare(a.Remote, b.Remote),
			cmp.Compare(a.PID, b.PID),
			cmp.Compare(a.Process, b.Process),
		)
	})
	return Table{Connections: rows, Limited: limited, Provider: "Linux procfs"}
}

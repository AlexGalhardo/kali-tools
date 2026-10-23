package snmpfake

import (
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gosnmp/gosnmp"
)

const (
	SysName = "kali-tools-lab"
	// WrongDigestOID is the USM report the v3 agent returns for a bad passphrase.
	WrongDigestOID = ".1.3.6.1.6.3.15.1.1.5.0"
)

// Agent is a read-only SNMPv2c responder. A v3 packet gets a wrong-digest report.
type Agent struct {
	Community string
	Rows      int
	Requests  atomic.Int32
	conn      *net.UDPConn
	done      chan struct{}
	once      sync.Once
}

func Start(community string, rows int) (*Agent, error) {
	return Listen("127.0.0.1:0", community, rows)
}

func Listen(addr, community string, rows int) (*Agent, error) {
	udp, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp4", udp)
	if err != nil {
		return nil, err
	}
	a := &Agent{Community: community, Rows: rows, conn: conn, done: make(chan struct{})}
	go a.serve()
	return a, nil
}

func (a *Agent) Addr() string { return a.conn.LocalAddr().String() }

func (a *Agent) Close() {
	a.once.Do(func() {
		close(a.done)
		_ = a.conn.Close()
	})
}

func (a *Agent) serve() {
	buf := make([]byte, 8192)
	for {
		n, addr, err := a.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		a.Requests.Add(1)
		reply := a.answer(buf[:n])
		if reply == nil {
			continue
		}
		_, _ = a.conn.WriteToUDP(reply, addr)
	}
}

func (a *Agent) answer(raw []byte) []byte {
	if isV3(raw) {
		return v3WrongDigest(raw)
	}
	client := &gosnmp.GoSNMP{Version: gosnmp.Version2c, Community: a.Community, Logger: gosnmp.NewLogger(log.New(io.Discard, "", 0))}
	pkt, err := client.SnmpDecodePacket(raw)
	if err != nil || pkt.Community != a.Community {
		return nil
	}
	vars := a.vars(pkt)
	out := gosnmp.SnmpPacket{
		Version: gosnmp.Version2c, Community: a.Community, PDUType: gosnmp.GetResponse,
		RequestID: pkt.RequestID, Variables: vars,
	}
	encoded, err := out.MarshalMsg()
	if err != nil {
		return nil
	}
	return encoded
}

func (a *Agent) vars(pkt *gosnmp.SnmpPacket) []gosnmp.SnmpPDU {
	table := a.table()
	if pkt.PDUType == gosnmp.GetBulkRequest {
		reps := int(pkt.MaxRepetitions)
		if reps < 1 {
			reps = 1
		}
		if reps > 32 {
			reps = 32
		}
		var out []gosnmp.SnmpPDU
		for _, variable := range pkt.Variables {
			out = append(out, children(table, variable.Name, reps)...)
		}
		if len(out) == 0 {
			return []gosnmp.SnmpPDU{{Name: ".1.3.6.1.2.1.2.2.1.2", Type: gosnmp.EndOfMibView, Value: nil}}
		}
		return out
	}
	out := make([]gosnmp.SnmpPDU, 0, len(pkt.Variables))
	for _, variable := range pkt.Variables {
		if pdu, ok := table[variable.Name]; ok {
			out = append(out, pdu)
			continue
		}
		out = append(out, gosnmp.SnmpPDU{Name: variable.Name, Type: gosnmp.NoSuchInstance, Value: nil})
	}
	return out
}

func (a *Agent) table() map[string]gosnmp.SnmpPDU {
	table := map[string]gosnmp.SnmpPDU{
		".1.3.6.1.2.1.1.1.0": {Name: ".1.3.6.1.2.1.1.1.0", Type: gosnmp.OctetString, Value: "lab fixture"},
		".1.3.6.1.2.1.1.2.0": {Name: ".1.3.6.1.2.1.1.2.0", Type: gosnmp.ObjectIdentifier, Value: ".1.3.6.1.4.1.8072.3.2.10"},
		".1.3.6.1.2.1.1.3.0": {Name: ".1.3.6.1.2.1.1.3.0", Type: gosnmp.TimeTicks, Value: uint32(100)},
		".1.3.6.1.2.1.1.4.0": {Name: ".1.3.6.1.2.1.1.4.0", Type: gosnmp.OctetString, Value: "lab"},
		".1.3.6.1.2.1.1.5.0": {Name: ".1.3.6.1.2.1.1.5.0", Type: gosnmp.OctetString, Value: SysName},
		".1.3.6.1.2.1.1.6.0": {Name: ".1.3.6.1.2.1.1.6.0", Type: gosnmp.OctetString, Value: "lab"},
	}
	mac := []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	for i := 1; i <= a.Rows; i++ {
		put := func(column string, value any, kind gosnmp.Asn1BER) {
			name := fmt.Sprintf("%s.%d", column, i)
			table[name] = gosnmp.SnmpPDU{Name: name, Type: kind, Value: value}
		}
		put(".1.3.6.1.2.1.2.2.1.2", fmt.Sprintf("if%d", i), gosnmp.OctetString)
		put(".1.3.6.1.2.1.2.2.1.8", 1, gosnmp.Integer)
		put(".1.3.6.1.2.1.2.2.1.5", uint32(1_000_000_000), gosnmp.Gauge32)
		put(".1.3.6.1.2.1.2.2.1.6", mac, gosnmp.OctetString)
		put(".1.3.6.1.2.1.31.1.1.1.15", uint32(1000), gosnmp.Gauge32)
	}
	return table
}

func children(table map[string]gosnmp.SnmpPDU, requested string, reps int) []gosnmp.SnmpPDU {
	column := requested
	if _, leaf := table[requested]; leaf {
		if dot := strings.LastIndex(requested, "."); dot > 0 {
			column = requested[:dot]
		}
	}
	prefix := column + "."
	names := make([]string, 0, len(table))
	for name := range table {
		if strings.HasPrefix(name, prefix) && oidAfter(name, requested) {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return oidAfter(names[j], names[i]) })
	var out []gosnmp.SnmpPDU
	for _, name := range names {
		if len(out) == reps {
			break
		}
		out = append(out, table[name])
	}
	if len(out) == 0 {
		return []gosnmp.SnmpPDU{{Name: requested, Type: gosnmp.EndOfMibView, Value: nil}}
	}
	return out
}

func oidAfter(name, requested string) bool {
	left, right := oidParts(name), oidParts(requested)
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] != right[i] {
			return left[i] > right[i]
		}
	}
	return len(left) > len(right)
}

func oidParts(name string) []int {
	var out []int
	for _, part := range strings.Split(strings.TrimPrefix(name, "."), ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

func isV3(raw []byte) bool {
	return len(raw) > 4 && raw[0] == 0x30 && raw[2] == 0x02 && raw[3] == 0x01 && raw[4] == 0x03
}

func v3WrongDigest(raw []byte) []byte {
	pkt := gosnmp.SnmpPacket{
		Version: gosnmp.Version3, MsgFlags: gosnmp.Reportable, SecurityModel: gosnmp.UserSecurityModel,
		SecurityParameters: &gosnmp.UsmSecurityParameters{AuthoritativeEngineID: "kali-tools-lab"},
		PDUType:            gosnmp.Report,
		Variables:          []gosnmp.SnmpPDU{{Name: WrongDigestOID, Type: gosnmp.Integer, Value: 1}},
	}
	encoded, err := pkt.MarshalMsg()
	if err != nil {
		return nil
	}
	_ = raw
	return encoded
}

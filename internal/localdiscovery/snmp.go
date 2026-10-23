package localdiscovery

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gosnmp/gosnmp"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

var (
	errSNMPLimit = errors.New("snmp row limit")
	systemOIDs   = []string{
		".1.3.6.1.2.1.1.1.0", ".1.3.6.1.2.1.1.2.0", ".1.3.6.1.2.1.1.3.0",
		".1.3.6.1.2.1.1.4.0", ".1.3.6.1.2.1.1.5.0", ".1.3.6.1.2.1.1.6.0",
	}
	systemKeys = []string{"sysDescr", "sysObjectID", "sysUpTime", "sysContact", "sysName", "sysLocation"}
	ifColumns  = []string{
		".1.3.6.1.2.1.2.2.1.2", ".1.3.6.1.2.1.2.2.1.8", ".1.3.6.1.2.1.2.2.1.5",
		".1.3.6.1.2.1.2.2.1.6", ".1.3.6.1.2.1.31.1.1.1.15",
	}
)

type SNMPRequest struct {
	Scope      Scope        `json:"scope"`
	Target     string       `json:"target"`
	Port       int          `json:"port"`
	Owned      bool         `json:"owned"`
	Version    string       `json:"version"`
	Community  Secret       `json:"-"`
	V3         *SNMPv3Creds `json:"-"`
	TimeoutMS  int          `json:"timeout_ms"`
	Interfaces bool         `json:"interfaces"`
}

type SNMPv3Creds struct {
	User         string
	AuthProtocol string
	AuthPass     Secret
	PrivProtocol string
	PrivPass     Secret
}

type SNMPResult struct {
	Target     string            `json:"target"`
	System     map[string]string `json:"system"`
	Interfaces []SNMPInterface   `json:"interfaces"`
	Limited    bool              `json:"limited"`
}

type SNMPInterface struct {
	Index      int    `json:"index"`
	Descr      string `json:"descr"`
	OperStatus string `json:"oper_status"`
	SpeedBPS   uint64 `json:"speed_bps"`
	MAC        string `json:"mac"`
}

func ValidateSNMP(r SNMPRequest, ifaces []network.InterfaceInfo) error {
	if err := ValidateScope(r.Scope, ifaces); err != nil {
		return err
	}
	addr, err := netip.ParseAddr(r.Target)
	if err != nil || !addr.Is4() || !scopePrefix(r.Scope.CIDR).Contains(addr) {
		return domain.Invalid("target must be an IPv4 address inside the scope")
	}
	if !r.Owned {
		return domain.Invalid("confirm that you own or administer this device")
	}
	if r.Port == 0 {
		r.Port = SNMPPort
	}
	if r.Port < 1 || r.Port > 65535 {
		return domain.Invalid("invalid port")
	}
	if r.TimeoutMS < SNMPMinTimeoutMS || r.TimeoutMS > SNMPMaxTimeoutMS {
		return domain.Invalid("timeout must be 500..5000 ms")
	}
	switch r.Version {
	case "v2c":
		if r.Community.reveal() == "" {
			return domain.Invalid("community is required")
		}
	case "v3":
		if r.V3 == nil || r.V3.User == "" || len(r.V3.User) > 64 || r.V3.AuthPass.reveal() == "" {
			return domain.Invalid("v3 user and auth passphrase are required")
		}
		if !allowed(r.V3.AuthProtocol, "SHA", "SHA256", "SHA512") {
			return domain.Invalid("invalid auth protocol")
		}
		if r.V3.PrivPass.reveal() != "" && !allowed(r.V3.PrivProtocol, "AES", "AES256") {
			return domain.Invalid("invalid privacy protocol")
		}
	default:
		return domain.Invalid("version must be v2c or v3")
	}
	return nil
}

func QuerySNMP(ctx context.Context, r SNMPRequest) (SNMPResult, error) {
	if r.Port == 0 {
		r.Port = SNMPPort
	}
	if err := ValidateSNMP(r, validateInterfaces()); err != nil {
		return SNMPResult{}, err
	}
	client := &gosnmp.GoSNMP{
		Target: r.Target, Port: uint16(r.Port), Timeout: time.Duration(r.TimeoutMS) * time.Millisecond,
		Retries: SNMPRetries, MaxOids: 6, MaxRepetitions: SNMPMaxRepetitions, Context: ctx,
		ExponentialTimeout: false, Logger: gosnmp.NewLogger(log.New(io.Discard, "", 0)),
	}
	if r.Version == "v2c" {
		client.Version = gosnmp.Version2c
		client.Community = r.Community.reveal()
	} else {
		client.Version = gosnmp.Version3
		client.SecurityModel = gosnmp.UserSecurityModel
		flags := gosnmp.AuthNoPriv
		priv := gosnmp.NoPriv
		pass := ""
		if r.V3.PrivPass.reveal() != "" {
			flags = gosnmp.AuthPriv
			priv = privacyProtocol(r.V3.PrivProtocol)
			pass = r.V3.PrivPass.reveal()
		}
		client.MsgFlags = flags
		client.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName: r.V3.User, AuthenticationProtocol: authProtocol(r.V3.AuthProtocol),
			AuthenticationPassphrase: r.V3.AuthPass.reveal(), PrivacyProtocol: priv, PrivacyPassphrase: pass,
		}
	}
	err := client.Connect()
	r.Community = Secret{}
	if r.V3 != nil {
		r.V3.AuthPass = Secret{}
		r.V3.PrivPass = Secret{}
	}
	if err != nil {
		return SNMPResult{}, snmpErr(err)
	}
	defer func() { _ = client.Conn.Close() }()
	pkt, err := client.Get(systemOIDs)
	if err != nil || pkt == nil {
		return SNMPResult{}, snmpErr(err)
	}
	if pkt.Error == gosnmp.AuthorizationError || pkt.Error == gosnmp.NoAccess {
		return SNMPResult{}, &domain.Error{Code: domain.CodeUnauthorized, Message: "SNMP credentials were rejected"}
	}
	out := SNMPResult{Target: r.Target, System: map[string]string{}, Interfaces: []SNMPInterface{}}
	for i, pdu := range pkt.Variables {
		if i < len(systemKeys) {
			out.System[systemKeys[i]] = snmpValue(pdu)
		}
	}
	if !r.Interfaces {
		return out, nil
	}
	byIndex := map[int]*SNMPInterface{}
	for _, column := range ifColumns {
		count := 0
		err = client.BulkWalk(column, func(pdu gosnmp.SnmpPDU) error {
			count++
			if count > SNMPMaxRows {
				out.Limited = true
				return errSNMPLimit
			}
			index := oidIndex(pdu.Name)
			row := byIndex[index]
			if row == nil {
				row = &SNMPInterface{Index: index, OperStatus: "unknown"}
				byIndex[index] = row
			}
			applyColumn(row, column, pdu)
			return nil
		})
		if err != nil && !errors.Is(err, errSNMPLimit) {
			return SNMPResult{}, snmpErr(err)
		}
	}
	indexes := make([]int, 0, len(byIndex))
	for index := range byIndex {
		indexes = append(indexes, index)
	}
	sortInts(indexes)
	for _, index := range indexes {
		out.Interfaces = append(out.Interfaces, *byIndex[index])
	}
	if len(out.Interfaces) > SNMPMaxRows {
		out.Interfaces = out.Interfaces[:SNMPMaxRows]
		out.Limited = true
	}
	return out, nil
}

func applyColumn(row *SNMPInterface, column string, pdu gosnmp.SnmpPDU) {
	switch column {
	case ifColumns[0]:
		row.Descr = snmpValue(pdu)
	case ifColumns[1]:
		row.OperStatus = operStatus(number(pdu.Value))
	case ifColumns[2]:
		if row.SpeedBPS == 0 {
			row.SpeedBPS = uint64(number(pdu.Value))
		}
	case ifColumns[3]:
		row.MAC = macString(pdu.Value)
	case ifColumns[4]:
		if n := number(pdu.Value); n > 0 {
			row.SpeedBPS = uint64(n) * 1_000_000
		}
	}
}

func snmpErr(err error) error {
	if err == nil {
		return &domain.Error{Code: domain.CodeFailed, Message: "SNMP query failed"}
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, gosnmp.ErrMaxRetriesExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return &domain.Error{Code: domain.CodeTimeout, Message: "SNMP agent did not answer"}
	}
	if errors.Is(err, gosnmp.ErrWrongDigest) || errors.Is(err, gosnmp.ErrUnknownUsername) || errors.Is(err, gosnmp.ErrDecryption) || errors.Is(err, gosnmp.ErrUnknownSecurityLevel) {
		return &domain.Error{Code: domain.CodeUnauthorized, Message: "SNMP credentials were rejected"}
	}
	text := err.Error()
	if strings.Contains(text, "authorization") || strings.Contains(text, "wrong digest") || strings.Contains(text, "usmStatsWrongDigests") {
		return &domain.Error{Code: domain.CodeUnauthorized, Message: "SNMP credentials were rejected"}
	}
	if strings.Contains(text, "timeout") || strings.Contains(text, "deadline") {
		return &domain.Error{Code: domain.CodeTimeout, Message: "SNMP agent did not answer"}
	}
	return &domain.Error{Code: domain.CodeFailed, Message: "SNMP query failed"}
}

func snmpValue(pdu gosnmp.SnmpPDU) string {
	if pdu.Type == gosnmp.TimeTicks {
		return (time.Duration(number(pdu.Value)) * 10 * time.Millisecond).String()
	}
	switch v := pdu.Value.(type) {
	case []byte:
		if utf8.Valid(v) && !bytesLookBinary(v) {
			return clip(string(v), SNMPMaxValueBytes)
		}
		return clip(hex.EncodeToString(v), SNMPMaxValueBytes)
	case string:
		if utf8.ValidString(v) {
			return clip(v, SNMPMaxValueBytes)
		}
		return clip(hex.EncodeToString([]byte(v)), SNMPMaxValueBytes)
	default:
		return clip(fmt.Sprint(v), SNMPMaxValueBytes)
	}
}

func bytesLookBinary(v []byte) bool {
	for _, b := range v {
		if b == 0 {
			return true
		}
	}
	return false
}

func macString(value any) string {
	raw, ok := value.([]byte)
	if !ok {
		if text, isText := value.(string); isText {
			raw = []byte(text)
		}
	}
	if len(raw) == 0 {
		return ""
	}
	parts := make([]string, len(raw))
	for i, b := range raw {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, ":")
}

func operStatus(n int) string {
	switch n {
	case 1:
		return "up"
	case 2:
		return "down"
	case 3:
		return "testing"
	case 4:
		return "unknown"
	case 5:
		return "dormant"
	case 6:
		return "notPresent"
	case 7:
		return "lowerLayerDown"
	default:
		return "unknown"
	}
}

func number(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case uint:
		return int(v)
	case uint32:
		return int(v)
	case uint64:
		return int(v)
	case int64:
		return int(v)
	default:
		n, _ := strconv.Atoi(fmt.Sprint(v))
		return n
	}
}

func oidIndex(name string) int {
	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return 0
	}
	n, _ := strconv.Atoi(name[dot+1:])
	return n
}

func sortInts(values []int) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func allowed(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func authProtocol(name string) gosnmp.SnmpV3AuthProtocol {
	switch name {
	case "SHA256":
		return gosnmp.SHA256
	case "SHA512":
		return gosnmp.SHA512
	default:
		return gosnmp.SHA
	}
}

func privacyProtocol(name string) gosnmp.SnmpV3PrivProtocol {
	if name == "AES256" {
		return gosnmp.AES256
	}
	return gosnmp.AES
}

package contract_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
)

var allDTOs = []any{contract.DNSInspectRequest{}, contract.DNSInspectReport{}, contract.DNSResolverAnswer{}, contract.DNSComparison{}, contract.DNSAuthServer{}, contract.DNSConsistency{}, contract.DNSSECLink{}, contract.DNSSECChain{}, contract.DNSEncryptedResult{}, contract.TLSAuditRequest{}, contract.TLSAuditReport{}, contract.TLSAuditCertificate{}, contract.TLSAuditFinding{}, contract.TLSAuditProtocolResult{}, contract.TLSAuditSCTSummary{}, contract.Neighbor{}, contract.NeighborTable{}, contract.Error{}, contract.Capability{}, contract.Interface{}, contract.Counter{}, contract.Adapter{}, contract.Inventory{}, contract.DiagnosticRequest{}, contract.DiagnosticResult{}, contract.HTTPMetadata{}, contract.HTTPPhases{}, contract.SecurityHeader{}, contract.RedirectTarget{}, contract.RedirectHop{}, contract.Job{}, contract.JobEvent{}, contract.Settings{}, contract.PortResult{}, contract.Packet{}, contract.Capture{}, contract.LiveCaptureRequest{}, contract.LiveCapture{}, contract.CaptureEvent{}, contract.Rate{}, contract.Metrics{}, contract.WiFi{}, contract.WiFiSecret{}, contract.Tool{}, contract.ServiceInspection{}, contract.InspectionFinished{}, contract.InspectionState{}, contract.InspectionAddress{}, contract.InspectionService{}, contract.InspectionPort{}, contract.InspectionHost{}}

func TestEveryDTOIsRedactionSafe(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var exported, registered []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.TypeSpec)
			if ok && spec.Name.IsExported() {
				if _, ok := spec.Type.(*ast.StructType); ok {
					exported = append(exported, spec.Name.Name)
				}
			}
			return true
		})
	}
	for _, dto := range allDTOs {
		typ := reflect.TypeOf(dto)
		registered = append(registered, typ.Name())
		checkFields(t, typ)
	}
	slices.Sort(exported)
	slices.Sort(registered)
	if !slices.Equal(exported, registered) {
		t.Fatalf("exported=%v registry=%v", exported, registered)
	}
}
func checkFields(t *testing.T, typ reflect.Type) {
	t.Helper()
	switch typ.Kind() {
	case reflect.Pointer:
		checkFields(t, typ.Elem())
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			t.Fatalf("raw bytes in %v", typ)
		}
		checkFields(t, typ.Elem())
	case reflect.Struct:
		if typ.PkgPath() == "time" {
			return
		}
		for field := range typ.Fields() {
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if slices.Contains([]string{"key", "psk", "password", "passphrase", "secret", "token", "payload", "raw", "material", "bytes"}, name) {
				t.Fatalf("forbidden field %s.%s", typ, name)
			}
			if name == "value" && typ != reflect.TypeFor[contract.WiFiSecret]() && typ != reflect.TypeFor[contract.SecurityHeader]() {
				t.Fatalf("secret value outside WiFiSecret: %v", typ)
			}
			checkFields(t, field.Type)
		}
	}
}
func TestWiFiSecretNeverFormatsValue(t *testing.T) {
	secret := contract.WiFiSecret{State: "available", Value: "synthetic-test-key"}
	for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
		if got := fmt.Sprintf(format, secret); strings.Contains(got, secret.Value) || !strings.Contains(got, "[redacted]") {
			t.Fatalf("%s: %s", format, got)
		}
	}
	if !strings.Contains(string(jsonBytes(t, secret)), secret.Value) {
		t.Fatal("explicit reveal value missing")
	}
}

func TestCaptureDTOsCarryNoPayload(t *testing.T) {
	artifact := capture.NewArtifact(layers.LinkTypeRaw)
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, SrcIP: []byte{192, 0, 2, 1}, DstIP: []byte{192, 0, 2, 2}, Protocol: layers.IPProtocolTCP}
	tcp := &layers.TCP{SrcPort: 12345, DstPort: 80, DataOffset: 5}
	buffer := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buffer, gopacket.SerializeOptions{FixLengths: true}, ip, tcp, gopacket.Payload("synthetic-private")); err != nil {
		t.Fatal(err)
	}
	if err := artifact.Add(buffer.Bytes(), gopacket.CaptureInfo{Timestamp: time.Unix(100, 0), CaptureLength: len(buffer.Bytes()), Length: len(buffer.Bytes())}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "synthetic.pcapng")
	if err := artifact.Save(t.Context(), path, "pcapng"); err != nil {
		t.Fatal(err)
	}
	imported, err := capture.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, dto := range []any{contract.FromImportResult(imported), contract.FromLiveResult(capture.LiveResult{Packets: imported.Packets})} {
		raw := string(jsonBytes(t, dto))
		if strings.Contains(raw, "synthetic-private") {
			t.Fatal(raw)
		}
		if !strings.Contains(raw, "192.0.2.1") {
			t.Fatal("missing decoded packet")
		}
	}
}

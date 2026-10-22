package contract_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/adapter/contract"
	"github.com/AlexGalhardo/kali-tools/internal/app"
	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/job"
	"github.com/AlexGalhardo/kali-tools/internal/network"
	"github.com/AlexGalhardo/kali-tools/internal/tlsaudit"
)

// Reflective fixtures exercise every source field, including nested slices and pointers.
// Production mappers remain explicit field-by-field copies.
func fixture(v reflect.Value, empty bool) {
	if v.Type() == reflect.TypeFor[time.Time]() {
		v.Set(reflect.ValueOf(time.Date(2026, 10, 21, 1, 2, 3, 0, time.UTC)))
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).Tag.Get("json") != "-" {
				fixture(v.Field(i), empty)
			}
		}
	case reflect.Slice:
		n := 1
		if empty {
			n = 0
		}
		v.Set(reflect.MakeSlice(v.Type(), n, n))
		if n > 0 {
			fixture(v.Index(0), empty)
		}
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fixture(v.Elem(), empty)
	case reflect.String:
		v.SetString("synthetic")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int32, reflect.Int64:
		v.SetInt(23)
	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		v.SetUint(42)
	case reflect.Float64:
		v.SetFloat(12.5)
	}
}

type mappingCase struct {
	name     string
	source   any
	mapValue func(any) any
}

func mapping[S, D any](f func(S) D) mappingCase {
	var source S
	return mappingCase{reflect.TypeFor[S]().String(), source, func(v any) any { return f(v.(S)) }}
}
func mappings() []mappingCase {
	return []mappingCase{
		mapping(contract.FromTLSAudit),
		mapping(func(v contract.TLSAuditRequest) tlsaudit.Request { return v.Request() }),
		mapping(contract.FromNeighborTable),
		mapping(contract.FromCapabilities), mapping(contract.FromInterfaces), mapping(contract.FromInventory),
		mapping(func(v network.Request) contract.DiagnosticRequest {
			return contract.DiagnosticRequest{RecordType: v.RecordType, Kind: v.Kind, Target: v.Target, Port: v.Port, TimeoutMS: v.TimeoutMS}
		}),
		mapping(contract.FromDiagnosticResult), mapping(contract.FromStatus), mapping(contract.FromStatuses), mapping(contract.FromSettings), mapping(contract.FromPortResults), mapping(contract.FromImportResult),
		mapping(func(v capture.LiveRequest) contract.LiveCaptureRequest {
			return contract.LiveCaptureRequest{Backend: v.Backend, Address: v.Address, DurationSeconds: v.DurationSeconds, PacketLimit: v.PacketLimit}
		}),
		mapping(contract.FromLiveResult), mapping(contract.FromMetrics), mapping(contract.FromWiFi), mapping(contract.FromWiFiSecret), mapping(contract.FromTools), mapping(contract.FromServiceInspection),
	}
}
func jsonBytes(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestDTOJSONMatchesSourceTypes(t *testing.T) {
	for _, test := range mappings() {
		t.Run(test.name, func(t *testing.T) {
			v := reflect.New(reflect.TypeOf(test.source)).Elem()
			fixture(v, false)
			source := v.Interface()
			got := test.mapValue(source)
			if string(jsonBytes(t, source)) != string(jsonBytes(t, got)) {
				t.Fatalf("source=%s\ndto=%s", jsonBytes(t, source), jsonBytes(t, got))
			}
		})
	}
}
func TestMappersPreserveNilAndEmptySlices(t *testing.T) {
	for _, test := range mappings() {
		for _, empty := range []bool{false, true} {
			v := reflect.New(reflect.TypeOf(test.source)).Elem()
			if empty {
				fixture(v, true)
			}
			if string(jsonBytes(t, v.Interface())) != string(jsonBytes(t, test.mapValue(v.Interface()))) {
				t.Fatalf("%s empty=%v differs", test.name, empty)
			}
		}
	}
}
func mutate(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			v.Elem().Set(reflect.Zero(v.Type().Elem()))
		}
	case reflect.Slice:
		if v.Len() > 0 {
			mutate(v.Index(0))
			v.Index(0).Set(reflect.Zero(v.Type().Elem()))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				mutate(v.Field(i))
			}
		}
	}
}
func TestMappersDoNotAlias(t *testing.T) {
	for _, test := range mappings() {
		t.Run(test.name, func(t *testing.T) {
			v := reflect.New(reflect.TypeOf(test.source)).Elem()
			fixture(v, false)
			dto := test.mapValue(v.Interface())
			before := string(jsonBytes(t, dto))
			mutate(v)
			if string(jsonBytes(t, dto)) != before {
				t.Fatal("DTO aliases source")
			}
		})
	}
}
func TestInputMappings(t *testing.T) {
	r := contract.DiagnosticRequest{Kind: "dns", Target: "example.test", RecordType: "A", Port: 53, TimeoutMS: 100}
	if got := r.Request(); got.Kind != r.Kind || got.Target != r.Target || got.RecordType != r.RecordType || got.Port != r.Port || got.TimeoutMS != r.TimeoutMS {
		t.Fatal(got)
	}
	s := contract.Settings{Language: "en", RetentionDays: 30}
	if s.AppSettings() != (app.Settings{Language: "en", RetentionDays: 30}) {
		t.Fatal(s)
	}
	l := contract.LiveCaptureRequest{Address: "192.0.2.1", DurationSeconds: 1, PacketLimit: 2}
	if l.LiveRequest() != (capture.LiveRequest{Address: l.Address, DurationSeconds: 1, PacketLimit: 2}) {
		t.Fatal(l)
	}
}
func TestEventPayload(t *testing.T) {
	status := job.Status{ID: "one"}
	result := network.Result{Kind: "dns"}
	for _, test := range []struct {
		event job.Event
		name  string
		ok    bool
	}{
		{job.Event{Topic: job.TopicJob, SchemaVersion: 1, Seq: 9, Job: &status, Data: &result}, "job", true},
		{job.Event{Topic: job.TopicJob, Job: &status}, "job", true},
		{job.Event{Topic: job.TopicJob, Job: &status, Data: (*network.Result)(nil)}, "job", true},
		{job.Event{Topic: job.TopicJob}, "", false},
		{job.Event{Topic: job.TopicJob, Job: &status, Data: "bad"}, "", false},
		{job.Event{Topic: job.TopicMetrics, Data: network.Metrics{}}, "metrics", true},
		{job.Event{Topic: job.TopicMetrics, Data: "bad"}, "", false},
		{job.Event{Topic: job.TopicCapture, Data: app.CaptureEvent{Error: "legacy"}}, "capture", true},
		{job.Event{Topic: job.TopicCapture, Data: app.CaptureEvent{Err: domain.Invalid("typed")}}, "capture", true},
		{job.Event{Topic: job.TopicCapture, Data: "bad"}, "", false},
		{job.Event{Topic: "unknown"}, "", false},
	} {
		name, payload, ok := contract.EventPayload(test.event)
		if name != test.name || ok != test.ok {
			t.Fatalf("%+v: %s %v", test.event, name, ok)
		}
		if p, ok := payload.(contract.JobEvent); ok {
			if p.Seq != test.event.Seq || p.SchemaVersion != test.event.SchemaVersion {
				t.Fatal(p)
			}
			encoded := string(jsonBytes(t, p))
			if (test.event.Data == &result) != (p.Result != nil) {
				t.Fatal(encoded)
			}
		}
		if p, ok := payload.(contract.CaptureEvent); ok {
			if outcome := test.event.Data.(app.CaptureEvent); outcome.Err != nil && p.Error != "invalid_input: typed" {
				t.Fatal(p)
			} else if outcome.Err == nil && p.Error != outcome.Error {
				t.Fatal(p)
			}
		}
	}
}

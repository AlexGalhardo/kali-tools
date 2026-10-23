// Package bundle writes a local diagnostic zip. It does not open the network.
package bundle

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const (
	SchemaVersion  = 1
	MaxLogs        = 50
	MaxBundleBytes = 1 << 20
	maxInput       = 8192
	maxField       = 512
	minSecretLen   = 8
	minNameLen     = 3
)

// Input is everything the zip may contain. Secrets are deleted.
// Names (hostname, SSIDs) become stable per-bundle pseudonyms.
// Scan results, capture packets and Wi-Fi keys are not fields of Input.
type Input struct {
	AppVersion   string
	GoVersion    string
	OS           string
	Arch         string
	Hostname     string
	Names        []string
	Secrets      []string
	CaptureNote  string
	WiFiNote     string
	Capabilities []Capability
	Tools        []Tool
	Settings     Settings
	Logs         []Log
}

type Capability struct {
	ID       string
	State    string
	Detail   string
	Provider string
}

type Tool struct {
	Name    string
	State   string
	Version string
	Path    string
}

type Settings struct {
	Language      string
	RetentionDays int
}

type Log struct {
	ID      string
	Kind    string
	State   string
	Summary string
	Created string
}

type report struct {
	SchemaVersion int          `json:"schema_version"`
	Versions      versions     `json:"versions"`
	Doctor        doctor       `json:"doctor"`
	Capabilities  []capability `json:"capabilities"`
	Settings      settings     `json:"settings"`
	Logs          []log        `json:"logs"`
}

type versions struct {
	App  string `json:"app"`
	Go   string `json:"go"`
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type doctor struct {
	OS           string       `json:"os"`
	Arch         string       `json:"arch"`
	Capabilities []capability `json:"capabilities"`
	Tools        []tool       `json:"tools"`
	Capture      string       `json:"capture"`
	WiFi         string       `json:"wifi"`
}

type capability struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Detail   string `json:"detail"`
	Provider string `json:"provider"`
}

type tool struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

type settings struct {
	Language      string `json:"language"`
	RetentionDays int    `json:"retention_days"`
}

type log struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Summary string `json:"summary"`
	Created string `json:"created"`
}

// Write builds summary.txt and report.json. Pseudonyms are stable for this call only.
func Write(w io.Writer, in Input) error {
	body, err := bytesOf(in)
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

func bytesOf(in Input) ([]byte, error) {
	red := newRedactor(in)
	caps := make([]capability, len(in.Capabilities))
	for i, item := range in.Capabilities {
		caps[i] = capability{ID: red.text(item.ID), State: red.text(item.State), Detail: red.text(item.Detail), Provider: red.text(item.Provider)}
	}
	tools := make([]tool, len(in.Tools))
	for i, item := range in.Tools {
		tools[i] = tool{Name: red.text(item.Name), State: red.text(item.State), Version: red.text(item.Version), Path: red.text(item.Path)}
	}
	logs := in.Logs
	if len(logs) > MaxLogs {
		logs = logs[:MaxLogs]
	}
	rows := make([]log, len(logs))
	for i, item := range logs {
		rows[i] = log{ID: red.text(item.ID), Kind: red.text(item.Kind), State: red.text(item.State), Summary: red.text(item.Summary), Created: red.text(item.Created)}
	}
	doc := doctor{
		OS: red.text(in.OS), Arch: red.text(in.Arch), Capabilities: caps, Tools: tools,
		Capture: red.text(in.CaptureNote), WiFi: red.text(in.WiFiNote),
	}
	payload := report{
		SchemaVersion: SchemaVersion,
		Versions:      versions{App: red.text(in.AppVersion), Go: red.text(in.GoVersion), OS: doc.OS, Arch: doc.Arch},
		Doctor:        doc,
		Capabilities:  caps,
		Settings:      settings{Language: red.text(in.Settings.Language), RetentionDays: in.Settings.RetentionDays},
		Logs:          rows,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	summary := "kali-tools diagnostic bundle\n" +
		"schema_version: 1\n" +
		"app: " + payload.Versions.App + "\n" +
		"go: " + payload.Versions.Go + "\n" +
		"os: " + payload.Versions.OS + "\n" +
		"arch: " + payload.Versions.Arch + "\n" +
		"capabilities: " + itoa(len(caps)) + "\n" +
		"logs: " + itoa(len(rows)) + "\n" +
		"Nothing in this archive was uploaded.\n"
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := addFile(zw, "summary.txt", []byte(summary)); err != nil {
		return nil, err
	}
	if err := addFile(zw, "report.json", raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if buf.Len() > MaxBundleBytes {
		return nil, domain.Invalid("diagnostic bundle is too large")
	}
	return buf.Bytes(), nil
}

func addFile(zw *zip.Writer, name string, body []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)}
	header.SetMode(0o644)
	writer, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = writer.Write(body)
	return err
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

type hit struct {
	start int
	end   int
	kind  string
	value string
}

type redactor struct {
	secrets []string
	names   []string
	tokens  map[string]string
	seq     map[string]int
}

func newRedactor(in Input) *redactor {
	secrets := make([]string, 0, len(in.Secrets))
	seen := map[string]bool{}
	for _, secret := range in.Secrets {
		if len(secret) < minSecretLen || seen[secret] {
			continue
		}
		seen[secret] = true
		secrets = append(secrets, secret)
	}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	names := make([]string, 0, len(in.Names)+1)
	if len(in.Hostname) >= minNameLen {
		names = append(names, in.Hostname)
	}
	for _, name := range in.Names {
		if len(name) >= minNameLen {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	return &redactor{secrets: secrets, names: names, tokens: map[string]string{}, seq: map[string]int{}}
}

func (r *redactor) text(s string) string {
	for _, secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, "")
	}
	s = clip(s, maxInput)
	hits := r.find(s)
	sort.Slice(hits, func(i, j int) bool { return hits[i].start < hits[j].start })
	var b strings.Builder
	cursor := 0
	for _, item := range hits {
		if item.start < cursor || item.end > len(s) {
			continue
		}
		b.WriteString(s[cursor:item.start])
		b.WriteString(r.token(item.kind, item.value))
		cursor = item.end
	}
	b.WriteString(s[cursor:])
	return clip(b.String(), maxField)
}

func (r *redactor) token(kind, value string) string {
	key := kind + "\x00" + value
	if token, ok := r.tokens[key]; ok {
		return token
	}
	r.seq[kind]++
	token := kind + itoa(r.seq[kind])
	r.tokens[key] = token
	return token
}

var (
	labeledName = regexp.MustCompile(`(?i)(?:\b(?:ssid|bssid|hostname)\s*[:=]\s*|"(?:ssid|bssid|hostname)"\s*:\s*")([^"\s]{1,32})`)
	macColon    = regexp.MustCompile(`(?i)\b[0-9a-f]{2}(?::[0-9a-f]{2}){5}\b`)
	macDash     = regexp.MustCompile(`(?i)\b[0-9a-f]{2}(?:-[0-9a-f]{2}){5}\b`)
	ipv4        = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`)
	dottedName  = regexp.MustCompile(`(?i)\b(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+(?:[a-z]{2,63})\b`)
)

func (r *redactor) find(s string) []hit {
	var hits []hit
	add := func(kind, value string, start, end int) {
		if start < 0 || end <= start || end > len(s) {
			return
		}
		for _, item := range hits {
			if start < item.end && end > item.start {
				return
			}
		}
		hits = append(hits, hit{start: start, end: end, kind: kind, value: value})
	}
	for _, match := range labeledName.FindAllStringSubmatchIndex(s, -1) {
		if len(match) < 4 || match[2] < 0 {
			continue
		}
		add("name", s[match[2]:match[3]], match[2], match[3])
	}
	for _, name := range r.names {
		offset := 0
		rest := s
		for {
			index := strings.Index(rest, name)
			if index < 0 {
				break
			}
			add("name", name, offset+index, offset+index+len(name))
			rest = rest[index+len(name):]
			offset += index + len(name)
		}
	}
	for _, match := range macColon.FindAllStringIndex(s, -1) {
		add("mac", strings.ToLower(s[match[0]:match[1]]), match[0], match[1])
	}
	for _, match := range macDash.FindAllStringIndex(s, -1) {
		add("mac", strings.ToLower(s[match[0]:match[1]]), match[0], match[1])
	}
	for _, match := range ipv4.FindAllStringIndex(s, -1) {
		add("ip", s[match[0]:match[1]], match[0], match[1])
	}
	hits = append(hits, findIPv6(s, hits)...)
	for _, match := range dottedName.FindAllStringIndex(s, -1) {
		add("name", strings.ToLower(s[match[0]:match[1]]), match[0], match[1])
	}
	return hits
}

func findIPv6(s string, existing []hit) []hit {
	var hits []hit
	occupied := func(start, end int) bool {
		for _, item := range existing {
			if start < item.end && end > item.start {
				return true
			}
		}
		for _, item := range hits {
			if start < item.end && end > item.start {
				return true
			}
		}
		return false
	}
	for i := 0; i < len(s); {
		if s[i] != ':' && !isHex(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && (s[j] == ':' || isHex(s[j])) {
			j++
		}
		candidate := s[i:j]
		if strings.Count(candidate, ":") >= 2 && net.ParseIP(candidate) != nil && !occupied(i, j) {
			hits = append(hits, hit{start: i, end: j, kind: "ip", value: strings.ToLower(candidate)})
			i = j
			continue
		}
		i++
	}
	return hits
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

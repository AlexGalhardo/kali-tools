package dnsinspect

import (
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"github.com/AlexGalhardo/kali-tools/internal/network"
)

const (
	MaxResolvers       = 4
	MaxDoH             = 2
	MaxDoT             = 2
	MaxNameservers     = 8
	MaxAddrsPerNS      = 2
	MaxTrustAnchors    = 4
	MaxChainDepth      = 8
	MaxQueries         = 64
	MaxAnswersReported = 64
	MaxResponseBytes   = 64 << 10
	MaxRDataText       = 1024
	Workers            = 8
	MinTimeoutMS       = 200
	MaxTimeoutMS       = 5000
	OverallDeadline    = 30 * time.Second
	EDNSBufferSize     = 1232
	maxDoHURLBytes     = 2048
)

// Request is a read-only DNS inspection. Checks names the subset to run.
type Request struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Resolvers    []string `json:"resolvers,omitempty"`
	DoH          []string `json:"doh,omitempty"`
	DoT          []string `json:"dot,omitempty"`
	Checks       []string `json:"checks"`
	TrustAnchors []string `json:"trust_anchors,omitempty"`
	TimeoutMS    int      `json:"timeout_ms"`
}

// Report is the inspection result. Fields other than name, type, queries and
// limited stay empty until that check is implemented.
type Report struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Compare   *Comparison       `json:"compare,omitempty"`
	SOANS     *Consistency      `json:"soa_ns,omitempty"`
	DNSSEC    *DNSSECChain      `json:"dnssec,omitempty"`
	Encrypted []EncryptedResult `json:"encrypted,omitempty"`
	Queries   int               `json:"queries"`
	Limited   bool              `json:"limited"`
}

// ResolverAnswer is one resolver's view of a name.
type ResolverAnswer struct {
	Resolver  string   `json:"resolver"`
	Transport string   `json:"transport"`
	Status    string   `json:"status"`
	RCode     string   `json:"rcode,omitempty"`
	AD        bool     `json:"ad"`
	Answers   []string `json:"answers"`
	MinTTL    *uint32  `json:"min_ttl,omitempty"`
	ElapsedMS float64  `json:"elapsed_ms"`
}

// Comparison reports whether resolver answers agree.
type Comparison struct {
	Results []ResolverAnswer `json:"results"`
	Agree   bool             `json:"agree"`
	Groups  [][]string       `json:"groups"`
}

// AuthServer is one authoritative server observed for a zone.
type AuthServer struct {
	NS      string   `json:"ns"`
	Address string   `json:"address"`
	Status  string   `json:"status"`
	Serial  *uint32  `json:"serial,omitempty"`
	NSSet   []string `json:"ns_set"`
	AA      bool     `json:"aa"`
}

// Consistency is the SOA and NS agreement verdict.
type Consistency struct {
	Zone         string       `json:"zone"`
	Servers      []AuthServer `json:"servers"`
	SerialsAgree bool         `json:"serials_agree"`
	NSSetsAgree  bool         `json:"ns_sets_agree"`
	Verdict      string       `json:"verdict"`
}

// DNSSECLink is one zone in a read-only chain.
type DNSSECLink struct {
	Zone      string `json:"zone"`
	DSMatches bool   `json:"ds_matches"`
	KeyTag    uint16 `json:"key_tag"`
	Algorithm string `json:"algorithm"`
	SigValid  bool   `json:"sig_valid"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}

// DNSSECChain is the observed validation result, not a trust decision.
type DNSSECChain struct {
	Links     []DNSSECLink `json:"links"`
	AnswerSig bool         `json:"answer_sig"`
	Verdict   string       `json:"verdict"`
	Resolver  string       `json:"resolver"`
}

// EncryptedResult is one DoH or DoT reachability observation.
type EncryptedResult struct {
	Endpoint  string  `json:"endpoint"`
	Protocol  string  `json:"protocol"`
	Status    string  `json:"status"`
	HTTPCode  int     `json:"http_code,omitempty"`
	RCode     string  `json:"rcode,omitempty"`
	ElapsedMS float64 `json:"elapsed_ms"`
}

var recordTypes = map[string]struct{}{
	"": {}, "A": {}, "AAAA": {}, "CNAME": {}, "MX": {}, "NS": {}, "TXT": {},
	"SOA": {}, "CAA": {}, "DS": {}, "DNSKEY": {},
}

var checkNames = map[string]struct{}{
	"compare": {}, "soa_ns": {}, "dnssec": {}, "encrypted": {},
}

// Validate checks Request bounds. An empty Type is accepted and means A.
// Network failures are not validation errors.
func Validate(r Request) error {
	if err := network.ValidateHost(r.Name); err != nil {
		return domain.Invalid("invalid name")
	}
	if _, err := netip.ParseAddr(r.Name); err == nil {
		return domain.Invalid("name must be a domain")
	}
	if _, ok := recordTypes[r.Type]; !ok {
		return domain.Invalid("unsupported record type")
	}
	if len(r.Resolvers) > MaxResolvers {
		return domain.Invalid("too many resolvers")
	}
	seenResolver := make(map[string]struct{}, len(r.Resolvers))
	for _, raw := range r.Resolvers {
		canonical, err := canonicalResolver(raw)
		if err != nil {
			return err
		}
		if _, ok := seenResolver[canonical]; ok {
			return domain.Invalid("duplicate resolver")
		}
		seenResolver[canonical] = struct{}{}
	}
	if len(r.DoH) > MaxDoH {
		return domain.Invalid("too many DoH endpoints")
	}
	for _, raw := range r.DoH {
		if err := validateDoH(raw); err != nil {
			return err
		}
	}
	if len(r.DoT) > MaxDoT {
		return domain.Invalid("too many DoT endpoints")
	}
	for _, raw := range r.DoT {
		if err := validateDoT(raw); err != nil {
			return err
		}
	}
	if len(r.Checks) == 0 {
		return domain.Invalid("at least one check is required")
	}
	seenCheck := make(map[string]struct{}, len(r.Checks))
	encrypted := false
	for _, check := range r.Checks {
		if _, ok := checkNames[check]; !ok {
			return domain.Invalid("unknown check")
		}
		if _, ok := seenCheck[check]; ok {
			return domain.Invalid("duplicate check")
		}
		seenCheck[check] = struct{}{}
		if check == "encrypted" {
			encrypted = true
		}
	}
	if encrypted && len(r.DoH) == 0 && len(r.DoT) == 0 {
		return domain.Invalid("encrypted transport needs an endpoint")
	}
	if len(r.TrustAnchors) > MaxTrustAnchors {
		return domain.Invalid("too many trust anchors")
	}
	if err := validateAnchors(r.TrustAnchors); err != nil {
		return err
	}
	if r.TimeoutMS < MinTimeoutMS || r.TimeoutMS > MaxTimeoutMS {
		return domain.Invalid("timeout must be 200..5000 ms")
	}
	return nil
}

func canonicalResolver(raw string) (string, error) {
	if raw == "" || strings.ContainsAny(raw, " \t\r\n") {
		return "", domain.Invalid("invalid resolver")
	}
	if ap, err := netip.ParseAddrPort(raw); err == nil {
		if err := zoneAllowed(ap.Addr()); err != nil {
			return "", err
		}
		return ap.String(), nil
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return "", domain.Invalid("invalid resolver")
	}
	if err := zoneAllowed(addr); err != nil {
		return "", err
	}
	return netip.AddrPortFrom(addr, 53).String(), nil
}

func zoneAllowed(addr netip.Addr) error {
	if addr.Zone() != "" && !addr.IsLinkLocalUnicast() && !addr.IsLinkLocalMulticast() {
		return domain.Invalid("invalid resolver")
	}
	return nil
}

func validateDoH(raw string) error {
	if raw == "" || len(raw) > maxDoHURLBytes || strings.ContainsAny(raw, " \t\r\n") {
		return domain.Invalid("invalid DoH endpoint")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return domain.Invalid("invalid DoH endpoint")
	}
	if u.Port() != "" {
		port, conv := strconv.Atoi(u.Port())
		if conv != nil || port < 1 || port > 65535 {
			return domain.Invalid("invalid DoH endpoint")
		}
	}
	if err := network.ValidateHost(u.Hostname()); err != nil {
		return domain.Invalid("invalid DoH endpoint")
	}
	return nil
}

func validateDoT(raw string) error {
	host := raw
	if h, p, err := net.SplitHostPort(raw); err == nil {
		host = h
		port, conv := strconv.Atoi(p)
		if conv != nil || port < 1 || port > 65535 {
			return domain.Invalid("invalid DoT endpoint")
		}
	}
	if host == "" || network.ValidateHost(host) != nil {
		return domain.Invalid("invalid DoT endpoint")
	}
	return nil
}

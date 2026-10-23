package contract

import (
	"slices"

	"github.com/AlexGalhardo/kali-tools/internal/dnsinspect"
)

func (r DNSInspectRequest) Request() dnsinspect.Request {
	return dnsinspect.Request{
		Name: r.Name, Type: r.Type, Resolvers: slices.Clone(r.Resolvers), DoH: slices.Clone(r.DoH), DoT: slices.Clone(r.DoT),
		Checks: slices.Clone(r.Checks), TrustAnchors: slices.Clone(r.TrustAnchors), TimeoutMS: r.TimeoutMS,
	}
}

func FromDNSInspect(v dnsinspect.Report) DNSInspectReport {
	out := DNSInspectReport{Name: v.Name, Type: v.Type, Queries: v.Queries, Limited: v.Limited, Encrypted: cloneEncrypted(v.Encrypted)}
	if v.Compare != nil {
		results := make([]DNSResolverAnswer, len(v.Compare.Results))
		for i, result := range v.Compare.Results {
			results[i] = DNSResolverAnswer{
				Resolver: result.Resolver, Transport: result.Transport, Status: result.Status, RCode: result.RCode,
				AD: result.AD, Answers: slices.Clone(result.Answers), MinTTL: cloneUint32(result.MinTTL), ElapsedMS: result.ElapsedMS,
			}
		}
		out.Compare = &DNSComparison{Results: results, Agree: v.Compare.Agree, Groups: cloneGroups(v.Compare.Groups)}
	}
	if v.SOANS != nil {
		servers := make([]DNSAuthServer, len(v.SOANS.Servers))
		for i, server := range v.SOANS.Servers {
			servers[i] = DNSAuthServer{NS: server.NS, Address: server.Address, Status: server.Status, Serial: cloneUint32(server.Serial), NSSet: slices.Clone(server.NSSet), AA: server.AA}
		}
		out.SOANS = &DNSConsistency{Zone: v.SOANS.Zone, Servers: servers, SerialsAgree: v.SOANS.SerialsAgree, NSSetsAgree: v.SOANS.NSSetsAgree, Verdict: v.SOANS.Verdict}
	}
	if v.DNSSEC != nil {
		links := make([]DNSSECLink, len(v.DNSSEC.Links))
		for i, link := range v.DNSSEC.Links {
			links[i] = DNSSECLink{Zone: link.Zone, DSMatches: link.DSMatches, KeyTag: link.KeyTag, Algorithm: link.Algorithm, SigValid: link.SigValid, Status: link.Status, Detail: link.Detail}
		}
		out.DNSSEC = &DNSSECChain{Links: links, AnswerSig: v.DNSSEC.AnswerSig, Verdict: v.DNSSEC.Verdict, Resolver: v.DNSSEC.Resolver}
	}
	return out
}

func cloneEncrypted(in []dnsinspect.EncryptedResult) []DNSEncryptedResult {
	if in == nil {
		return nil
	}
	out := make([]DNSEncryptedResult, len(in))
	for i, item := range in {
		out[i] = DNSEncryptedResult{Endpoint: item.Endpoint, Protocol: item.Protocol, Status: item.Status, HTTPCode: item.HTTPCode, RCode: item.RCode, ElapsedMS: item.ElapsedMS}
	}
	return out
}

func cloneGroups(in [][]string) [][]string {
	if in == nil {
		return nil
	}
	out := make([][]string, len(in))
	for i := range in {
		out[i] = slices.Clone(in[i])
	}
	return out
}

func cloneUint32(v *uint32) *uint32 {
	if v == nil {
		return nil
	}
	n := *v
	return &n
}

type DNSInspectRequest struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Resolvers    []string `json:"resolvers,omitempty"`
	DoH          []string `json:"doh,omitempty"`
	DoT          []string `json:"dot,omitempty"`
	Checks       []string `json:"checks"`
	TrustAnchors []string `json:"trust_anchors,omitempty"`
	TimeoutMS    int      `json:"timeout_ms"`
}

type DNSInspectReport struct {
	Name      string               `json:"name"`
	Type      string               `json:"type"`
	Compare   *DNSComparison       `json:"compare,omitempty"`
	SOANS     *DNSConsistency      `json:"soa_ns,omitempty"`
	DNSSEC    *DNSSECChain         `json:"dnssec,omitempty"`
	Encrypted []DNSEncryptedResult `json:"encrypted,omitempty"`
	Queries   int                  `json:"queries"`
	Limited   bool                 `json:"limited"`
}

type DNSResolverAnswer struct {
	Resolver  string   `json:"resolver"`
	Transport string   `json:"transport"`
	Status    string   `json:"status"`
	RCode     string   `json:"rcode,omitempty"`
	AD        bool     `json:"ad"`
	Answers   []string `json:"answers"`
	MinTTL    *uint32  `json:"min_ttl,omitempty"`
	ElapsedMS float64  `json:"elapsed_ms"`
}

type DNSComparison struct {
	Results []DNSResolverAnswer `json:"results"`
	Agree   bool                `json:"agree"`
	Groups  [][]string          `json:"groups"`
}

type DNSAuthServer struct {
	NS      string   `json:"ns"`
	Address string   `json:"address"`
	Status  string   `json:"status"`
	Serial  *uint32  `json:"serial,omitempty"`
	NSSet   []string `json:"ns_set"`
	AA      bool     `json:"aa"`
}

type DNSConsistency struct {
	Zone         string          `json:"zone"`
	Servers      []DNSAuthServer `json:"servers"`
	SerialsAgree bool            `json:"serials_agree"`
	NSSetsAgree  bool            `json:"ns_sets_agree"`
	Verdict      string          `json:"verdict"`
}

type DNSSECLink struct {
	Zone      string `json:"zone"`
	DSMatches bool   `json:"ds_matches"`
	KeyTag    uint16 `json:"key_tag"`
	Algorithm string `json:"algorithm"`
	SigValid  bool   `json:"sig_valid"`
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
}

type DNSSECChain struct {
	Links     []DNSSECLink `json:"links"`
	AnswerSig bool         `json:"answer_sig"`
	Verdict   string       `json:"verdict"`
	Resolver  string       `json:"resolver"`
}

type DNSEncryptedResult struct {
	Endpoint  string  `json:"endpoint"`
	Protocol  string  `json:"protocol"`
	Status    string  `json:"status"`
	HTTPCode  int     `json:"http_code,omitempty"`
	RCode     string  `json:"rcode,omitempty"`
	ElapsedMS float64 `json:"elapsed_ms"`
}

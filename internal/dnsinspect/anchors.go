package dnsinspect

import (
	"strings"

	"github.com/miekg/dns"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// Root anchors checked against https://data.iana.org/root-anchors/root-anchors.xml
// on 2026-10-23. Key tags 20326 and 38696 matched these digests. The retired
// tag 19036 remains in the file with validUntil 2019-01-11 and is not used.
const (
	rootAnchor20326 = ". IN DS 20326 8 2 E06D44B80B8F1D39A95C0B0D7C65D08458E880409BBC683457104237C7F8EC8D"
	rootAnchor38696 = ". IN DS 38696 8 2 683D2D0ACB8C9B712A1948B27F741219298D0A450D612C483AF444A4C0FB2B16"
)

func rootAnchors() []string { return []string{rootAnchor20326, rootAnchor38696} }

func validateAnchors(anchors []string) error {
	for _, raw := range anchors {
		rr, err := dns.NewRR(raw)
		if err != nil {
			return domain.Invalid("invalid trust anchor")
		}
		if _, ok := rr.(*dns.DS); !ok {
			return domain.Invalid("invalid trust anchor")
		}
	}
	return nil
}

func parseAnchors(raw []string) ([]*dns.DS, error) {
	if len(raw) == 0 {
		raw = rootAnchors()
	}
	out := make([]*dns.DS, 0, len(raw))
	owner := ""
	for _, text := range raw {
		rr, err := dns.NewRR(text)
		if err != nil {
			return nil, domain.Invalid("invalid trust anchor")
		}
		ds, ok := rr.(*dns.DS)
		if !ok {
			return nil, domain.Invalid("invalid trust anchor")
		}
		name := strings.ToLower(ds.Hdr.Name)
		if owner == "" {
			owner = name
		} else if name != owner {
			return nil, domain.Invalid("trust anchors must share an owner")
		}
		out = append(out, ds)
	}
	return out, nil
}

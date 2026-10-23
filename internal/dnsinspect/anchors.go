package dnsinspect

import (
	"github.com/miekg/dns"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

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

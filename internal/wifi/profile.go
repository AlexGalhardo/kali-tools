package wifi

import (
	"encoding/xml"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func secretFromProfile(raw []byte) (Secret, error) {
	if len(raw) > 128<<10 {
		return Secret{}, domain.Invalid("WLAN profile exceeds limit")
	}
	var profile struct {
		XMLName xml.Name `xml:"WLANProfile"`
		Key     struct {
			Protected bool   `xml:"protected"`
			Material  string `xml:"keyMaterial"`
		} `xml:"MSM>security>sharedKey"`
	}
	if e := xml.Unmarshal(raw, &profile); e != nil {
		return Secret{}, domain.Invalid("invalid WLAN profile")
	}
	if profile.Key.Protected {
		return Secret{State: "permission_required"}, nil
	}
	if profile.Key.Material == "" {
		return Secret{State: "no_saved_key"}, nil
	}
	return Secret{State: "available", Value: profile.Key.Material}, nil
}

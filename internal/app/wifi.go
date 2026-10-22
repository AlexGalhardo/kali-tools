package app

import (
	"errors"
	"sync"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

type wifiObservation struct {
	mu    sync.Mutex
	state string
}

func (o *wifiObservation) record(err error) {
	state := domain.CapabilityAvailable
	if err != nil {
		var failure *domain.Error
		if !errors.As(err, &failure) || failure == nil {
			return
		}
		if failure.Code != domain.CodeLocationConsentRequired && failure.Code != domain.CodePermissionRequired {
			return
		}
		state = failure.Code
	}
	o.mu.Lock()
	o.state = state
	o.mu.Unlock()
}

func (o *wifiObservation) snapshot() (string, string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	detail := "Native Wi-Fi request succeeded"
	switch o.state {
	case domain.CapabilityLocationConsentRequired:
		detail = "Windows location access is off for this app; open " + domain.LocationSettingsURI
	case domain.CapabilityPermissionRequired:
		detail = "Windows denied WLAN access"
	}
	return o.state, detail
}

package wifi

import (
	"fmt"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// Windows11_24H2Build is the first build with the WLAN location-consent gate.
const Windows11_24H2Build = 26100
const errorAccessDenied = 5
const consentMessage = "Windows location access is off for this app. Open Settings > Privacy & security > Location (" + domain.LocationSettingsURI + "), allow location for desktop apps, then try again."

func currentConnectionError(code uintptr, build uint32) error {
	if code == errorAccessDenied {
		if build >= Windows11_24H2Build {
			return &domain.Error{Code: domain.CodeLocationConsentRequired, Message: consentMessage}
		}
		return &domain.Error{Code: domain.CodePermissionRequired, Message: "Windows denied WLAN access"}
	}
	return &domain.Error{Code: domain.CodeFailed, Message: fmt.Sprintf("WLAN API returned %d", code)}
}

func revealState(securityEnabled, oneX uint32) (string, bool) {
	if securityEnabled == 0 {
		return "open_network", true
	}
	if oneX != 0 {
		return "enterprise", true
	}
	return "", false
}

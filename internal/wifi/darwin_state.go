package wifi

import (
	"fmt"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const coreWLANLocationMessage = "macOS did not provide the Wi-Fi network name. Allow Location Services for this app, then try again."

func networkFromCoreWLAN(code int, ssid, bssid string, channel, rssi int) (Network, error) {
	switch code {
	case 1:
		return Network{}, domain.Unsupported("No Wi-Fi interface")
	case 2:
		return Network{}, &domain.Error{Code: domain.CodeLocationConsentRequired, Message: coreWLANLocationMessage}
	case 0:
	default:
		return Network{}, &domain.Error{Code: domain.CodeFailed, Message: fmt.Sprintf("CoreWLAN returned %d", code)}
	}
	if ssid == "" || len(ssid) > 32 {
		return Network{}, domain.Invalid("invalid Wi-Fi network name")
	}
	network := Network{SSID: ssid, State: "connected"}
	if len(bssid) <= 17 {
		network.BSSID = bssid
	}
	if channel > 0 && channel <= 233 {
		value := uint32(channel)
		network.Channel = &value
	}
	if rssi >= 0 && rssi <= 100 {
		network.Signal = uint32(rssi)
	}
	return network, nil
}

func secretFromKeychain(status int, value string) (Secret, error) {
	switch status {
	case 0:
		if value == "" {
			return Secret{State: "no_saved_key"}, nil
		}
		if len(value) > 256 {
			return Secret{}, domain.Invalid("Wi-Fi key exceeds limit")
		}
		return Secret{State: "available", Value: value}, nil
	case -25300:
		return Secret{State: "no_saved_key"}, nil
	case -128, -25293, -25308:
		return Secret{}, &domain.Error{Code: domain.CodePermissionRequired, Message: "macOS did not allow Keychain access without a prompt"}
	case -34018:
		return Secret{}, domain.Unsupported("This build cannot read the Wi-Fi key from Keychain")
	case 3:
		return Secret{}, domain.Invalid("Wi-Fi key exceeds limit")
	default:
		return Secret{}, domain.Unsupported(fmt.Sprintf("macOS Keychain status %d", status))
	}
}

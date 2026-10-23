//go:build linux

package wifi

import (
	"context"
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/mdlayher/wifi"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const maxWiFiInterfaces = 64

func Current() (Network, error) {
	client, err := wifi.New()
	if err != nil {
		return Network{}, nlError(err)
	}
	defer func() { _ = client.Close() }()
	ifaces, err := client.Interfaces()
	if err != nil {
		return Network{}, nlError(err)
	}
	if len(ifaces) > maxWiFiInterfaces {
		return Network{}, domain.Invalid("Wi-Fi interface count exceeds limit")
	}
	for _, iface := range ifaces {
		if iface == nil || (iface.Type != wifi.InterfaceTypeStation && iface.Type != wifi.InterfaceTypeP2PClient && iface.Type != wifi.InterfaceTypeUnspecified) {
			continue
		}
		bss, err := client.BSS(iface)
		if err != nil || bss == nil || bss.Status != wifi.BSSStatusAssociated || bss.SSID == "" {
			continue
		}
		network := Network{SSID: bss.SSID, State: "connected", Signal: signalPercent(bss.Signal, bss.SignalUnspecified)}
		if bss.BSSID != nil {
			network.BSSID = bss.BSSID.String()
		}
		if channel := channelFromMHz(bss.Frequency); channel > 0 {
			network.Channel = &channel
		}
		return network, nil
	}
	return Network{}, &domain.Error{Code: domain.CodeNoConnection, Message: "No connected Wi-Fi interface"}
}

func Reveal() (Secret, error) {
	current, err := Current()
	if err != nil {
		return Secret{}, err
	}
	return nmSecret(context.Background(), current.SSID)
}

func signalPercent(milliDBm int32, unspecified uint32) uint32 {
	if unspecified > 0 && unspecified <= 100 {
		return unspecified
	}
	dbm := int(milliDBm / 100)
	if dbm >= -50 {
		return 100
	}
	if dbm <= -100 {
		return 0
	}
	return uint32((dbm + 100) * 2)
}

func channelFromMHz(mhz int) uint32 {
	switch {
	case mhz == 2484:
		return 14
	case mhz >= 2412 && mhz <= 2472 && (mhz-2412)%5 == 0:
		return uint32((mhz-2412)/5 + 1)
	case mhz >= 5000 && mhz <= 5885 && (mhz-5000)%5 == 0:
		return uint32((mhz - 5000) / 5)
	default:
		return 0
	}
}

func nlError(err error) error {
	text := err.Error()
	switch {
	case errors.Is(err, errors.ErrUnsupported) || strings.Contains(text, "not supported") || strings.Contains(text, "no such"):
		return domain.Unsupported("nl80211 is not available")
	case strings.Contains(text, "permission denied") || strings.Contains(text, "operation not permitted"):
		return &domain.Error{Code: domain.CodePermissionRequired, Message: "nl80211 denied Wi-Fi metadata"}
	default:
		return &domain.Error{Code: domain.CodeFailed, Message: "nl80211 Wi-Fi query failed"}
	}
}

func nmSecret(ctx context.Context, ssid string) (Secret, error) {
	if err := ctx.Err(); err != nil {
		return Secret{}, err
	}
	if ssid == "" || len(ssid) > 32 {
		return Secret{}, domain.Invalid("Wi-Fi SSID is missing or too long")
	}
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return Secret{}, nmError(err)
	}
	defer func() { _ = conn.Close() }()
	bus := conn.Object("org.freedesktop.NetworkManager", "/org/freedesktop/NetworkManager")
	value, err := bus.GetProperty("org.freedesktop.NetworkManager.ActiveConnections")
	if err != nil {
		return Secret{}, nmError(err)
	}
	paths, ok := value.Value().([]dbus.ObjectPath)
	if !ok {
		return Secret{}, &domain.Error{Code: domain.CodeFailed, Message: "NetworkManager connection list was unreadable"}
	}
	if len(paths) > maxWiFiInterfaces {
		return Secret{}, domain.Invalid("NetworkManager connection count exceeds limit")
	}
	for _, path := range paths {
		secret, matched, err := connectionSecret(conn, path, ssid)
		if err != nil {
			return Secret{}, err
		}
		if matched {
			return secret, nil
		}
	}
	return Secret{State: "no_saved_key"}, nil
}

func connectionSecret(conn *dbus.Conn, path dbus.ObjectPath, ssid string) (Secret, bool, error) {
	active := conn.Object("org.freedesktop.NetworkManager", path)
	value, err := active.GetProperty("org.freedesktop.NetworkManager.Connection.Active.Connection")
	if err != nil {
		return Secret{}, false, nmError(err)
	}
	settingsPath, ok := value.Value().(dbus.ObjectPath)
	if !ok || settingsPath == "/" {
		return Secret{}, false, nil
	}
	settings := conn.Object("org.freedesktop.NetworkManager", settingsPath)
	var config map[string]map[string]dbus.Variant
	if err := settings.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSettings", 0).Store(&config); err != nil {
		return Secret{}, false, nmError(err)
	}
	wireless := config["802-11-wireless"]
	if variantText(wireless["ssid"]) != ssid {
		return Secret{}, false, nil
	}
	var secrets map[string]map[string]dbus.Variant
	if err := settings.Call("org.freedesktop.NetworkManager.Settings.Connection.GetSecrets", 0, "802-11-wireless-security").Store(&secrets); err != nil {
		return Secret{}, false, nmError(err)
	}
	security := map[string]string{}
	for key, variant := range secrets["802-11-wireless-security"] {
		security[key] = variantText(variant)
	}
	return secretFromNM(security), true, nil
}

func variantText(v dbus.Variant) string {
	switch value := v.Value().(type) {
	case string:
		if len(value) > 256 {
			return ""
		}
		return value
	case []byte:
		if len(value) > 256 {
			return ""
		}
		return strings.TrimRight(string(value), "\x00")
	default:
		return ""
	}
}

func secretFromNM(fields map[string]string) Secret {
	mgmt := fields["key-mgmt"]
	if strings.Contains(mgmt, "eap") || mgmt == "ieee8021x" {
		return Secret{State: "enterprise"}
	}
	material := fields["psk"]
	if material == "" {
		material = fields["sae-password"]
	}
	if material == "" {
		material = fields["wep-key0"]
	}
	if material == "" {
		if mgmt == "" || mgmt == "none" || mgmt == "wpa-none" {
			return Secret{State: "open_network"}
		}
		return Secret{State: "no_saved_key"}
	}
	if len(material) > 256 {
		return Secret{State: "no_saved_key"}
	}
	return Secret{State: "available", Value: material}
}

func nmError(err error) error {
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "serviceunknown"), strings.Contains(text, "namehasnoowner"), strings.Contains(text, "not provided"), strings.Contains(text, "no such file"), strings.Contains(text, "connection refused"):
		return &domain.Error{Code: domain.CodeDependencyMissing, Message: "NetworkManager is not available"}
	case strings.Contains(text, "accessdenied"), strings.Contains(text, "notauthorized"), strings.Contains(text, "permissiondenied"):
		return &domain.Error{Code: domain.CodePermissionRequired, Message: "NetworkManager denied secret access"}
	default:
		return &domain.Error{Code: domain.CodeFailed, Message: "NetworkManager secret request failed"}
	}
}

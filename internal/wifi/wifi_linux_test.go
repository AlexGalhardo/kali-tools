//go:build linux

package wifi

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestSignalAndChannel(t *testing.T) {
	if signalPercent(-5500, 0) != 90 || signalPercent(-4000, 0) != 100 || signalPercent(-11000, 0) != 0 || signalPercent(-9000, 40) != 40 {
		t.Fatal(signalPercent(-5500, 0), signalPercent(-9000, 40))
	}
	if channelFromMHz(2412) != 1 || channelFromMHz(2472) != 13 || channelFromMHz(2484) != 14 || channelFromMHz(5180) != 36 || channelFromMHz(2400) != 0 {
		t.Fatal(channelFromMHz(2412), channelFromMHz(5180))
	}
}

func TestSecretFromNetworkManager(t *testing.T) {
	secret := secretFromNM(map[string]string{"key-mgmt": "wpa-psk", "psk": "synthetic-psk"})
	if secret.State != "available" || secret.Value != "synthetic-psk" {
		t.Fatal(secret)
	}
	for _, fields := range []map[string]string{
		{"key-mgmt": "wpa-eap"},
		{"key-mgmt": "none"},
		{"key-mgmt": "sae"},
		{"key-mgmt": "wpa-psk", "psk": strings.Repeat("x", 257)},
	} {
		got := secretFromNM(fields)
		if got.Value != "" || (got.State != "enterprise" && got.State != "open_network" && got.State != "no_saved_key") {
			t.Fatal(fields, got)
		}
	}
	err := nmError(errors.New("org.freedesktop.DBus.Error.ServiceUnknown: synthetic-psk"))
	var target *domain.Error
	if !errors.As(err, &target) || target.Code != domain.CodeDependencyMissing || strings.Contains(target.Error(), "synthetic-psk") {
		t.Fatal(err)
	}
	err = nmError(errors.New("org.freedesktop.NetworkManager.Settings.PermissionDenied synthetic-psk"))
	if !errors.As(err, &target) || target.Code != domain.CodePermissionRequired || strings.Contains(target.Error(), "synthetic-psk") {
		t.Fatal(err)
	}
}

func TestLinuxWiFiCurrentIsExplicit(t *testing.T) {
	network, err := Current()
	if err == nil {
		if network.State != "connected" || strings.Contains(network.SSID, "\n") {
			t.Fatal(network)
		}
		return
	}
	var target *domain.Error
	if !errors.As(err, &target) || strings.Contains(target.Error(), "/") {
		t.Fatal(err)
	}
	switch target.Code {
	case domain.CodeNoConnection, domain.CodeUnsupported, domain.CodePermissionRequired, domain.CodeFailed:
	default:
		t.Fatal(target)
	}
}

type stalledObject struct{ dbus.BusObject }

func (stalledObject) CallWithContext(ctx context.Context, _ string, _ dbus.Flags, _ ...any) *dbus.Call {
	<-ctx.Done()
	return &dbus.Call{Err: ctx.Err()}
}

func TestConnectionSecretHonoursDeadlineAndCancellation(t *testing.T) {
	object := func(dbus.ObjectPath) dbus.BusObject { return stalledObject{} }
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	_, _, err := connectionSecret(ctx, object, "/a", "lab")
	var target *domain.Error
	if err = nmContextError(ctx, err); !errors.As(err, &target) || target.Code != domain.CodeTimeout || target.Message != "authorization prompt not answered" {
		t.Fatalf("deadline: %v", err)
	}
	cancelled, stop := context.WithCancel(t.Context())
	stop()
	_, _, err = connectionSecret(cancelled, object, "/a", "lab")
	if err = nmContextError(cancelled, err); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := nmSecret(cancelled, "lab"); !errors.Is(err, context.Canceled) {
		t.Fatalf("nmSecret cancel: %v", err)
	}
}

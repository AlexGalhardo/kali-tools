package wifi

import (
	"errors"
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestNetworkFromCoreWLAN(t *testing.T) {
	network, err := networkFromCoreWLAN(0, "example", "00:00:5e:00:53:01", 6, -50)
	if err != nil || network.SSID != "example" || network.BSSID != "00:00:5e:00:53:01" || network.State != "connected" || network.Signal != 0 || network.Channel == nil || *network.Channel != 6 {
		t.Fatalf("network=%+v err=%v", network, err)
	}
	network, err = networkFromCoreWLAN(0, "example", "00:00:5e:00:53:01:extra", 0, 40)
	if err != nil || network.BSSID != "" || network.Signal != 40 || network.Channel != nil {
		t.Fatalf("network=%+v err=%v", network, err)
	}
	_, err = networkFromCoreWLAN(2, "", "", 0, 0)
	var denied *domain.Error
	if !errors.As(err, &denied) || denied.Code != domain.CodeLocationConsentRequired {
		t.Fatal(err)
	}
	_, err = networkFromCoreWLAN(1, "", "", 0, 0)
	if !errors.As(err, &denied) || denied.Code != domain.CodeUnsupported {
		t.Fatal(err)
	}
	if _, err = networkFromCoreWLAN(0, "", "", 0, 0); err == nil {
		t.Fatal("empty network name accepted")
	}
}

func TestSecretFromKeychain(t *testing.T) {
	secret, err := secretFromKeychain(0, "synthetic-psk")
	if err != nil || secret.State != "available" || secret.Value != "synthetic-psk" {
		t.Fatalf("secret=%+v err=%v", secret, err)
	}
	secret, err = secretFromKeychain(-25300, "synthetic-psk")
	if err != nil || secret.State != "no_saved_key" || secret.Value != "" {
		t.Fatalf("missing=%+v err=%v", secret, err)
	}
	_, err = secretFromKeychain(-34018, "synthetic-psk")
	var denied *domain.Error
	if !errors.As(err, &denied) || denied.Code != domain.CodeUnsupported || denied.Message == "synthetic-psk" {
		t.Fatal(err)
	}
	_, err = secretFromKeychain(-25308, "synthetic-psk")
	if !errors.As(err, &denied) || denied.Code != domain.CodePermissionRequired {
		t.Fatal(err)
	}
	_, err = secretFromKeychain(99, "synthetic-psk")
	if !errors.As(err, &denied) || denied.Code != domain.CodeUnsupported {
		t.Fatal(err)
	}
	if _, err = secretFromKeychain(3, "synthetic-psk"); err == nil {
		t.Fatal("oversized key accepted")
	}
}

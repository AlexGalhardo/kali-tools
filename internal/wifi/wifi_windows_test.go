//go:build windows

package wifi

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

func TestWindowsWLANLayoutAndMetadata(t *testing.T) {
	if unsafe.Sizeof(iface{}) != 532 || unsafe.Sizeof(association{}) != 68 || unsafe.Sizeof(connection{}) != 604 {
		t.Fatal("WLAN native structure layout mismatch")
	}
	c := connection{}
	copy(c.Profile[:], []uint16{'T', 'e', 's', 't'})
	copy(c.Association.SSID[:], "Fixture")
	c.Association.SSIDLength = 7
	c.Association.Signal = 75
	c.Association.BSSID = [6]byte{2, 0, 0, 0, 0, 1}
	channel := uint32(36)
	n, err := networkFromConnection(c, &channel)
	if err != nil || n.SSID != "Fixture" || n.Profile != "Test" || n.BSSID != "02:00:00:00:00:01" || n.Signal != 75 || n.Channel == nil || *n.Channel != 36 {
		t.Fatalf("invalid synthetic metadata: %+v %v", n, err)
	}
	n, err = networkFromConnection(c, nil)
	if err != nil || n.Channel != nil {
		t.Fatal("unavailable channel must stay absent")
	}
	c.Association.SSIDLength = 33
	if _, err = networkFromConnection(c, nil); err == nil {
		t.Fatal("oversized SSID accepted")
	}
	c.Association.SSIDLength = 7
	c.Association.Signal = 101
	if _, err = networkFromConnection(c, nil); err == nil {
		t.Fatal("invalid signal accepted")
	}
}

func TestWLANErrorStates(t *testing.T) {
	for _, test := range []struct {
		name string
		code uintptr
		want string
	}{
		{name: "denied", code: 5, want: "permission_required"},
		{name: "service_inactive", code: 1062, want: "unsupported"},
		{name: "unknown", code: 12345, want: "failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var typed *domain.Error
			if err := wlanError(test.code); !errors.As(err, &typed) || typed.Code != test.want {
				t.Fatalf("error=%v, want %s", err, test.want)
			}
		})
	}
}

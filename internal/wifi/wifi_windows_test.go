//go:build windows

package wifi

import (
	"testing"
	"unsafe"
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

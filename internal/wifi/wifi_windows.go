//go:build windows

package wifi

import (
	"fmt"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
	"golang.org/x/sys/windows"
	"net"
	"unsafe"
)

var wlan = windows.NewLazySystemDLL("wlanapi.dll")
var openHandle = wlan.NewProc("WlanOpenHandle")
var closeHandle = wlan.NewProc("WlanCloseHandle")
var enumInterfaces = wlan.NewProc("WlanEnumInterfaces")
var queryInterface = wlan.NewProc("WlanQueryInterface")
var getProfile = wlan.NewProc("WlanGetProfile")
var freeMemory = wlan.NewProc("WlanFreeMemory")

type iface struct {
	GUID        windows.GUID
	Description [256]uint16
	State       uint32
}
type association struct {
	SSIDLength uint32
	SSID       [32]byte
	BSSType    uint32
	BSSID      [6]byte
	_          [2]byte
	PHYType    uint32
	PHYIndex   uint32
	Signal     uint32
	RX         uint32
	TX         uint32
}
type connection struct {
	State           uint32
	Mode            uint32
	Profile         [256]uint16
	Association     association
	SecurityEnabled uint32
	OneX            uint32
	Auth            uint32
	Cipher          uint32
}

func wlanError(code uintptr) error {
	if code == 5 {
		return &domain.Error{Code: "permission_required", Message: "Windows denied WLAN access"}
	}
	return &domain.Error{Code: "failed", Message: fmt.Sprintf("WLAN API returned %d", code)}
}
func connected() (windows.Handle, iface, connection, error) {
	var h windows.Handle
	var negotiated uint32
	code, _, _ := openHandle.Call(2, 0, uintptr(unsafe.Pointer(&negotiated)), uintptr(unsafe.Pointer(&h)))
	if code != 0 {
		return 0, iface{}, connection{}, wlanError(code)
	}
	var ptr unsafe.Pointer
	code, _, _ = enumInterfaces.Call(uintptr(h), 0, uintptr(unsafe.Pointer(&ptr)))
	if code != 0 {
		closeHandle.Call(uintptr(h), 0)
		return 0, iface{}, connection{}, wlanError(code)
	}
	defer freeMemory.Call(uintptr(ptr))
	count := *(*uint32)(ptr)
	if count > 256 {
		closeHandle.Call(uintptr(h), 0)
		return 0, iface{}, connection{}, domain.Invalid("WLAN interface count exceeds limit")
	}
	for _, i := range unsafe.Slice((*iface)(unsafe.Add(ptr, 8)), int(count)) {
		if i.State != 1 {
			continue
		}
		var data unsafe.Pointer
		var size, opcode uint32
		code, _, _ = queryInterface.Call(uintptr(h), uintptr(unsafe.Pointer(&i.GUID)), 7, 0, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&opcode)))
		if code != 0 {
			closeHandle.Call(uintptr(h), 0)
			return 0, iface{}, connection{}, wlanError(code)
		}
		if size < uint32(unsafe.Sizeof(connection{})) {
			freeMemory.Call(uintptr(data))
			closeHandle.Call(uintptr(h), 0)
			return 0, iface{}, connection{}, domain.Invalid("short WLAN connection result")
		}
		c := *(*connection)(data)
		freeMemory.Call(uintptr(data))
		return h, i, c, nil
	}
	closeHandle.Call(uintptr(h), 0)
	return 0, iface{}, connection{}, &domain.Error{Code: "no_connection", Message: "No connected Wi-Fi interface"}
}
func Current() (Network, error) {
	h, i, c, e := connected()
	if e != nil {
		return Network{}, e
	}
	defer closeHandle.Call(uintptr(h), 0)
	var data unsafe.Pointer
	var size, opcode uint32
	var channel *uint32
	code, _, _ := queryInterface.Call(uintptr(h), uintptr(unsafe.Pointer(&i.GUID)), 8, 0, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&opcode)))
	if code == 0 && data != nil {
		defer freeMemory.Call(uintptr(data))
		if size >= 4 {
			value := *(*uint32)(data)
			if value > 0 {
				channel = &value
			}
		}
	}
	return networkFromConnection(c, channel)
}
func Reveal() (Secret, error) {
	h, i, c, e := connected()
	if e != nil {
		return Secret{}, e
	}
	defer closeHandle.Call(uintptr(h), 0)
	if c.SecurityEnabled == 0 {
		return Secret{State: "open_network"}, nil
	}
	if c.OneX != 0 {
		return Secret{State: "enterprise"}, nil
	}
	var text *uint16
	flags := uint32(4)
	var access uint32
	code, _, _ := getProfile.Call(uintptr(h), uintptr(unsafe.Pointer(&i.GUID)), uintptr(unsafe.Pointer(&c.Profile[0])), 0, uintptr(unsafe.Pointer(&text)), uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&access)))
	if code != 0 {
		return Secret{}, wlanError(code)
	}
	defer freeMemory.Call(uintptr(unsafe.Pointer(text)))
	// The OS owns this NUL-terminated allocation; impose a defensive length limit.
	units := make([]uint16, 0, 4096)
	for j := 0; j < 65536; j++ {
		v := *(*uint16)(unsafe.Add(unsafe.Pointer(text), j*2))
		if v == 0 {
			break
		}
		units = append(units, v)
		if j == 65535 {
			return Secret{}, domain.Invalid("WLAN profile exceeds limit")
		}
	}
	return secretFromProfile([]byte(windows.UTF16ToString(units)))
}

func networkFromConnection(c connection, channel *uint32) (Network, error) {
	n := c.Association.SSIDLength
	if n > 32 || c.Association.Signal > 100 {
		return Network{}, domain.Invalid("invalid WLAN metadata")
	}
	return Network{Profile: windows.UTF16ToString(c.Profile[:]), SSID: string(c.Association.SSID[:n]), BSSID: net.HardwareAddr(c.Association.BSSID[:]).String(), Signal: c.Association.Signal, Authentication: c.Auth, State: "connected", Channel: channel}, nil
}

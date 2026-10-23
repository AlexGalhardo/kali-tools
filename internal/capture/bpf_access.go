package capture

import (
	"strings"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// bpfDevicePath accepts only /dev/bpf followed by digits.
func bpfDevicePath(name string) bool {
	rest, ok := cutBPF(name)
	if !ok || rest == "" {
		return false
	}
	for _, c := range rest {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func cutBPF(name string) (string, bool) {
	return strings.CutPrefix(name, "/dev/bpf")
}

func bpfAccessCapability(deviceCount int, readable bool) domain.Capability {
	c := domain.Capability{ID: "capture_live", Provider: "bpf", State: domain.CapabilityUnsupported}
	switch {
	case deviceCount <= 0:
		c.Detail = "No /dev/bpf device is present. kali-tools does not create BPF devices or change their permissions."
	case !readable:
		c.State = domain.CapabilityPermissionRequired
		c.Detail = "This user cannot open /dev/bpf. Add the user to the access_bpf group with the ChmodBPF package, then log in again. kali-tools never changes device permissions."
	default:
		c.Detail = "This user can open /dev/bpf. Live capture is not implemented. kali-tools never changes device permissions."
	}
	return c
}

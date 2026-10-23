//go:build integration

package dnsinspect

import (
	"crypto/x509"
	"net/netip"
)

// NewForLab installs the port override and TLS roots used by the Docker lab.
// IP keys select the port for that address. Other keys are name-server names,
// because glue A records cannot carry a port and the fixture has one address.
// Production Inspect does not call this.
func NewForLab(authPorts map[string]uint16, roots *x509.CertPool) {
	byIP := map[string]uint16{}
	byNS := map[string]uint16{}
	for key, port := range authPorts {
		if _, err := netip.ParseAddr(key); err == nil {
			byIP[key] = port
			continue
		}
		byNS[apex(key)] = port
	}
	inspectHook = func(ins *inspector) {
		ins.roots = roots
		ins.nsPort = byNS
		ins.authPort = func(ip netip.Addr) uint16 {
			if port, ok := byIP[ip.String()]; ok {
				return port
			}
			return 53
		}
	}
}

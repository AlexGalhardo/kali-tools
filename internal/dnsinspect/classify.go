package dnsinspect

import (
	"errors"
	"net"
	"syscall"
)

// classifyTransport reports why a UDP query failed after an optional TCP probe.
// A nil tcpErr means the TCP connect succeeded. The caller performs that connect.
func classifyTransport(port int, udpErr, tcpErr error) string {
	if isTimeout(udpErr) && tcpErr == nil {
		return "timeout"
	}
	if isTimeout(udpErr) && (isTimeout(tcpErr) || isRefused(tcpErr)) {
		if port == 53 {
			return "port53_unreachable"
		}
		return "unreachable"
	}
	return "unreachable"
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isRefused(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED)
}

// netTimeout is a timeout that does not carry a query or address.
type netTimeout struct{}

func (netTimeout) Error() string   { return "timeout" }
func (netTimeout) Timeout() bool   { return true }
func (netTimeout) Temporary() bool { return true }

package dnsinspect

import (
	"syscall"
	"testing"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestClassifyTransport(t *testing.T) {
	timeout := timeoutErr{}
	refused := syscall.ECONNREFUSED
	tests := []struct {
		name string
		port int
		udp  error
		tcp  error
		want string
	}{
		{name: "port 53 timeout and refused", port: 53, udp: timeout, tcp: refused, want: "port53_unreachable"},
		{name: "other port timeout and refused", port: 5357, udp: timeout, tcp: refused, want: "unreachable"},
		{name: "port 53 timeout and tcp up", port: 53, udp: timeout, tcp: nil, want: "timeout"},
		{name: "both timeouts on port 53", port: 53, udp: timeout, tcp: timeout, want: "port53_unreachable"},
		{name: "udp refused is not a block", port: 53, udp: refused, tcp: refused, want: "unreachable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyTransport(tt.port, tt.udp, tt.tcp); got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

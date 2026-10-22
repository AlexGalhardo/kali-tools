//go:build !windows

package pcap

func DetectNpcap() (NpcapStatus, error) { return NpcapStatus{}, nil }
func isElevated() bool                  { return false }

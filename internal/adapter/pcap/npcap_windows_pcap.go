//go:build windows && pcap

package pcap

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"time"

	libpcap "github.com/gopacket/gopacket/pcap"

	"github.com/AlexGalhardo/kali-tools/internal/capture"
	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

const backendBuilt = true

func (n *Npcap) Capture(ctx context.Context, r capture.LiveRequest) (capture.LiveResult, *capture.Artifact, error) {
	result := capture.LiveResult{Packets: []capture.Packet{}, Limits: []string{}}
	if err := ctx.Err(); err != nil {
		return result, nil, err
	}
	if r.Backend != capture.BackendNpcap {
		return result, nil, domain.Invalid("Npcap requires the npcap backend")
	}
	if err := capture.ValidateLiveRequest(r); err != nil {
		return result, nil, err
	}
	if status := n.Status(ctx); status.State != domain.CapabilityAvailable {
		return result, nil, &domain.Error{Code: status.State, Message: status.Detail}
	}
	all, err := libpcap.FindAllDevs()
	if err != nil {
		return result, nil, &domain.Error{Code: domain.CodeFailed, Message: "Npcap device enumeration failed"}
	}
	if len(all) > MaxPcapDevices {
		return result, nil, &domain.Error{Code: domain.CodeFailed, Message: "device list exceeds limit"}
	}
	devices := make([]device, 0, len(all))
	for _, entry := range all {
		d := device{Name: entry.Name}
		for _, a := range entry.Addresses {
			if address, ok := netip.AddrFromSlice(a.IP); ok {
				d.Addresses = append(d.Addresses, address.Unmap())
			}
		}
		devices = append(devices, d)
	}
	address, _ := netip.ParseAddr(r.Address)
	name, err := selectDevice(devices, address)
	if err != nil {
		return result, nil, err
	}
	inactive, err := libpcap.NewInactiveHandle(name)
	if err != nil {
		return result, nil, openError(err)
	}
	defer inactive.CleanUp()
	for _, configure := range []func() error{
		func() error { return inactive.SetSnapLen(NpcapSnapLen) },
		func() error { return inactive.SetPromisc(false) },
		func() error { return inactive.SetTimeout(NpcapReadTimeout) },
		func() error { return inactive.SetImmediateMode(true) },
		func() error { return inactive.SetBufferSize(NpcapBufferBytes) },
	} {
		if err := configure(); err != nil {
			return result, nil, openError(err)
		}
	}
	handle, err := inactive.Activate()
	if err != nil {
		return result, nil, openError(err)
	}
	defer handle.Close()
	artifact := capture.NewArtifact(handle.LinkType())
	status, err := n.detect()
	if err != nil {
		return result, nil, &domain.Error{Code: domain.CodeFailed, Message: "Npcap detection failed"}
	}
	result.Provider = "Npcap " + status.Version + " (gopacket/pcap)"
	result.Started = time.Now().UTC()
	session, stop := context.WithTimeout(ctx, time.Duration(r.DurationSeconds)*time.Second)
	defer stop()
	var captureErr error
	for session.Err() == nil && len(result.Packets) < r.PacketLimit {
		data, info, err := handle.ReadPacketData()
		if errors.Is(err, libpcap.NextErrorTimeoutExpired) {
			continue
		}
		if err != nil {
			captureErr = &domain.Error{Code: domain.CodeFailed, Message: "Npcap packet read failed"}
			break
		}
		if err := artifact.Add(data, info); err != nil {
			result.Limited = true
			break
		}
		packet, err := capture.Decode(data, handle.LinkType(), info, len(result.Packets)+1)
		if err != nil {
			captureErr = &domain.Error{Code: domain.CodeFailed, Message: "Npcap packet decode failed"}
			break
		}
		result.Packets = append(result.Packets, packet)
		if info.Length > info.CaptureLength && len(result.Limits) == 0 {
			result.Limits = append(result.Limits, capture.LimitTruncated)
		}
	}
	result.Limited = result.Limited || len(result.Packets) == r.PacketLimit
	stats, err := handle.Stats()
	if err == nil && stats.PacketsDropped >= 0 && stats.PacketsIfDropped >= 0 {
		dropped := uint64(stats.PacketsDropped) + uint64(stats.PacketsIfDropped)
		result.Dropped = &dropped
	} else {
		result.Limits = append(result.Limits, capture.LimitDropCountMissing)
	}
	result.Finished = time.Now().UTC()
	if ctx.Err() != nil {
		return result, artifact, ctx.Err()
	}
	return result, artifact, captureErr
}

func openError(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "access is denied") {
		return &domain.Error{Code: domain.CodePermissionRequired, Message: "Npcap could not open the device. Start kali-tools from an elevated terminal. kali-tools never elevates itself and never redistributes Npcap."}
	}
	return &domain.Error{Code: domain.CodeFailed, Message: "Npcap could not open the device"}
}

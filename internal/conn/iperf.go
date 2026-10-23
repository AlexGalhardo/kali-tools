package conn

import (
	"encoding/json"
	"math"
	"net"
	"strconv"

	"github.com/AlexGalhardo/kali-tools/internal/domain"
)

// ParseIperfJSON reads an iperf3 -J document. It does not execute iperf3 and
// does not copy bits_per_second. The result is not wire-compatible with iperf3.
func ParseIperfJSON(data []byte) (Sample, error) {
	if len(data) == 0 || len(data) > maxFileBytes {
		return Sample{}, domain.Invalid("iperf report is empty or too large")
	}
	var doc struct {
		Start struct {
			ConnectingTo struct {
				Host string `json:"host"`
				Port int    `json:"port"`
			} `json:"connecting_to"`
		} `json:"start"`
		End struct {
			SumSent struct {
				Bytes   int64   `json:"bytes"`
				Seconds float64 `json:"seconds"`
			} `json:"sum_sent"`
		} `json:"end"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Sample{}, domain.Invalid("iperf report is not a JSON object")
	}
	if doc.Start.ConnectingTo.Host == "" || doc.End.SumSent.Bytes < 0 || doc.Start.ConnectingTo.Port < 0 || doc.Start.ConnectingTo.Port > 65535 {
		return Sample{}, domain.Invalid("iperf report is missing an endpoint or byte count")
	}
	endpoint := doc.Start.ConnectingTo.Host
	if doc.Start.ConnectingTo.Port > 0 {
		endpoint = net.JoinHostPort(endpoint, strconv.Itoa(doc.Start.ConnectingTo.Port))
	}
	var duration int64
	if doc.End.SumSent.Seconds > 0 && !math.IsNaN(doc.End.SumSent.Seconds) && !math.IsInf(doc.End.SumSent.Seconds, 0) {
		duration = int64(math.Round(doc.End.SumSent.Seconds * 1000))
	}
	return Sample{Endpoint: endpoint, DurationMS: duration, ByteCount: uint64(doc.End.SumSent.Bytes), Source: "iperf3"}, nil
}

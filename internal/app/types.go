package app

import "github.com/AlexGalhardo/kali-tools/internal/network"

type Settings struct {
	Language      string `json:"language"`
	RetentionDays int    `json:"retention_days"`
	// LegalNoticeAck is the acknowledged notice version; only AcceptLegalNotice changes it.
	LegalNoticeAck string `json:"legal_notice_ack,omitempty"`
}

type HostAdapter struct {
	Index    uint32   `json:"index"`
	Name     string   `json:"name"`
	Gateways []string `json:"gateways"`
	DNS      []string `json:"dns"`
}

type HostSnapshot struct {
	Adapters []HostAdapter     `json:"adapters"`
	Counters []network.Counter `json:"counters"`
}

package app

import (
	"testing"

	"github.com/AlexGalhardo/kali-tools/internal/job"
)

func TestLocalDiscoverySummaries(t *testing.T) {
	if got := jobSummary("local_discovery", job.Completed, "2 responders, 1 out of scope"); got != "Local discovery completed: 2 responders, 1 out of scope" {
		t.Fatal(got)
	}
	if got := jobSummary("snmp", job.Failed, "0 interfaces"); got != "SNMP query failed: 0 interfaces" {
		t.Fatal(got)
	}
}

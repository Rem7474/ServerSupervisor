package handlers

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

// goldenPath is the shared protocol contract fixture, produced by the agent
// module (agent/internal/sender/contract_test.go) and consumed here. It holds a
// fully-populated agent report with every field set, so decoding it proves the
// server knows every field the agent can emit.
const goldenPath = "../../../protocol/agent_report.golden.json"

// TestAgentReportContract is the server half of the agent↔server protocol
// contract. It decodes the agent-produced golden report into models.AgentReport
// with DisallowUnknownFields, so the test FAILS the moment the agent emits a
// JSON key the server does not model — at any nesting depth. This catches the
// class of silent data-loss bugs where an agent field is dropped server-side
// because of a missing field or a renamed JSON tag.
//
// If this test fails after an intentional protocol change: add the missing
// field (or fix the tag) on the server model, then regenerate the golden with
// `go test ./internal/sender -run TestReportContractGolden -update` in agent/.
func TestAgentReportContract(t *testing.T) {
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read protocol golden %s: %v", goldenPath, err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var report models.AgentReport
	if err := dec.Decode(&report); err != nil {
		t.Fatalf("agent report does not decode losslessly into models.AgentReport — "+
			"the agent sends a field the server does not model (missing field or renamed json tag): %v", err)
	}

	// Spot-check representative deep fields actually populated, so a future
	// refactor cannot make the decode silently no-op on nested structures.
	// These three in particular drifted historically.
	if report.Metrics == nil || report.Metrics.CPUModel == "" {
		t.Error("metrics did not decode (top-level typed payload missing)")
	}
	if len(report.DiskHealth) == 0 || report.DiskHealth[0].PowerCycles == 0 || report.DiskHealth[0].ReallocSectors == 0 {
		t.Error("disk_health SMART fields (power_cycles / realloc_sectors) did not decode")
	}
	if report.WebLogs == nil || report.WebLogs.Threats == nil {
		t.Fatal("web_logs.threats did not decode")
	}
	if len(report.WebLogs.Threats.CrowdSecTopBlocked) == 0 || report.WebLogs.Threats.CrowdSecTopBlocked[0].Type == "" {
		t.Error("crowdsec_top_blocked[].type did not decode (CrowdSec decision type would be lost)")
	}
	if len(report.WebLogs.Threats.TopSuspiciousIPs) == 0 || report.WebLogs.Threats.TopSuspiciousIPs[0].BlockedType == "" {
		t.Error("top_suspicious_ips[].blocked_type did not decode")
	}
}

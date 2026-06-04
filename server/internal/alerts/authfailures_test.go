package alerts

import (
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/proxmoxclient"
)

func TestIsAuthFailureSyslogLine(t *testing.T) {
	positives := []proxmoxclient.PVESyslogLine{
		{T: "May 6 12:00:00 host sshd[1]: Failed password for root from 1.2.3.4"},
		{Msg: "pam_unix(sshd:auth): authentication failure; user=admin"},
		{Msg: "Invalid user oracle from 10.0.0.1"},
		{T: "Too many authentication failures for root"},
		{Msg: "Maximum authentication attempts exceeded"},
		// Case-insensitive matching.
		{Msg: "FAILED PASSWORD for backup"},
	}
	for _, line := range positives {
		if !isAuthFailureSyslogLine(line) {
			t.Errorf("expected auth-failure detection for %+v", line)
		}
	}

	negatives := []proxmoxclient.PVESyslogLine{
		{},
		{Msg: "Accepted password for root from 1.2.3.4"},
		{T: "session opened for user root"},
		{Level: "info", Tag: "systemd"},
	}
	for _, line := range negatives {
		if isAuthFailureSyslogLine(line) {
			t.Errorf("did not expect auth-failure detection for %+v", line)
		}
	}
}

func TestFormatSyslogLineText(t *testing.T) {
	tests := []struct {
		name string
		line proxmoxclient.PVESyslogLine
		want string
	}{
		{"prefers T", proxmoxclient.PVESyslogLine{T: "  full text  ", Msg: "msg"}, "full text"},
		{"falls back to Msg", proxmoxclient.PVESyslogLine{Msg: "  the message "}, "the message"},
		{"falls back to tag+level", proxmoxclient.PVESyslogLine{Tag: "sshd", Level: "err"}, "sshd err"},
		{"empty line -> empty", proxmoxclient.PVESyslogLine{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSyslogLineText(tt.line); got != tt.want {
				t.Errorf("formatSyslogLineText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEstimateAuthFailureLimit(t *testing.T) {
	// Non-positive window -> floor.
	if got := estimateAuthFailureLimit(time.Now().Add(time.Hour)); got != 300 {
		t.Errorf("future since: want floor 300, got %d", got)
	}
	// Small window -> floor 300.
	if got := estimateAuthFailureLimit(time.Now().Add(-1 * time.Minute)); got != 300 {
		t.Errorf("1m window: want floor 300, got %d", got)
	}
	// Large window -> capped at 5000.
	if got := estimateAuthFailureLimit(time.Now().Add(-100 * time.Hour)); got != 5000 {
		t.Errorf("100h window: want cap 5000, got %d", got)
	}
	// Mid window scales between the bounds.
	got := estimateAuthFailureLimit(time.Now().Add(-30 * time.Minute))
	if got < 300 || got > 5000 {
		t.Errorf("30m window: want within [300,5000], got %d", got)
	}
}

func TestExtractSyslogTimestamp(t *testing.T) {
	tests := []struct {
		text   string
		want   string
		wantOk bool
	}{
		{"May 6 12:34:56 pve sshd[1]: Failed password", "May 6 12:34:56", true},
		{"Dec 25 00:00:01 host msg", "Dec 25 00:00:01", true},
		{"  May 6 12:34:56 leading space", "May 6 12:34:56", true},
		{"2024-05-06T12:34:56Z iso format", "", false},
		{"garbage line", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := extractSyslogTimestamp(tt.text)
		if ok != tt.wantOk || got != tt.want {
			t.Errorf("extractSyslogTimestamp(%q) = (%q,%v), want (%q,%v)", tt.text, got, ok, tt.want, tt.wantOk)
		}
	}
}

func TestSyslogLineTimeEpoch(t *testing.T) {
	// Epoch seconds path (value > 2000-01-01).
	sec := int64(1_700_000_000) // 2023-11-14T...
	ts, ok := syslogLineTime(proxmoxclient.PVESyslogLine{Time: sec})
	if !ok {
		t.Fatal("expected epoch-seconds parse to succeed")
	}
	if ts.Year() != 2023 {
		t.Errorf("epoch seconds: want year 2023, got %d", ts.Year())
	}

	// Epoch milliseconds path (already in ms).
	ms := int64(1_700_000_000_000)
	tms, ok := syslogLineTime(proxmoxclient.PVESyslogLine{Time: ms})
	if !ok {
		t.Fatal("expected epoch-ms parse to succeed")
	}
	if !tms.Equal(ts) {
		t.Errorf("epoch ms and s should resolve to same instant: %v vs %v", tms, ts)
	}

	// No usable timestamp.
	if _, ok := syslogLineTime(proxmoxclient.PVESyslogLine{}); ok {
		t.Error("empty line should not yield a timestamp")
	}
}

func TestCountAuthFailuresInLines(t *testing.T) {
	base := time.Date(2023, 11, 14, 0, 0, 0, 0, time.UTC).Unix()
	since := time.Date(2023, 11, 13, 0, 0, 0, 0, time.UTC)

	lines := []proxmoxclient.PVESyslogLine{
		{Time: base, Msg: "Failed password for root"},                                       // counts
		{Time: base, Msg: "Accepted password for root"},                                     // not an auth failure
		{Time: base, Msg: "authentication failure for admin"},                               // counts
		{Time: 0, Msg: "Failed password but no timestamp"},                                  // skipped: no usable ts
		{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix(), Msg: "invalid user old"}, // before since
	}

	if got := countAuthFailuresInLines(lines, since); got != 2 {
		t.Errorf("countAuthFailuresInLines = %d, want 2", got)
	}
}

func TestAuthFailureLogLines(t *testing.T) {
	base := time.Date(2023, 11, 14, 0, 0, 0, 0, time.UTC).Unix()
	since := time.Date(2023, 11, 13, 0, 0, 0, 0, time.UTC)

	lines := []proxmoxclient.PVESyslogLine{
		{Time: base, T: "Failed password for root from 1.2.3.4"},
		{Time: base, Msg: "Accepted password for root"}, // filtered out (not a failure)
	}

	out := authFailureLogLines(lines, since, "pve1")
	if len(out) != 1 {
		t.Fatalf("want 1 formatted line, got %d (%v)", len(out), out)
	}
	if out[0] != "[pve1] Failed password for root from 1.2.3.4" {
		t.Errorf("unexpected formatted line: %q", out[0])
	}

	// Without a node name, no bracket prefix is added.
	outNoNode := authFailureLogLines(lines, since, "")
	if len(outNoNode) != 1 || outNoNode[0] != "Failed password for root from 1.2.3.4" {
		t.Errorf("unexpected line without node prefix: %v", outNoNode)
	}
}

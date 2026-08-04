package collector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/serversupervisor/agent/internal/config"
)

func TestCollectResticStatus_BinaryMissing(t *testing.T) {
	cfg := &config.Config{ResticBin: "/nonexistent/restic-binary-that-does-not-exist"}
	status, err := CollectResticStatus(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Installed {
		t.Errorf("expected Installed=false, got true")
	}
	if status.Source != "unavailable" {
		t.Errorf("expected Source=unavailable, got %q", status.Source)
	}
}

func TestReadResticProfileStatusFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")

	content := `{
		"profiles": {
			"files": {
				"backup": {"success": true, "time": "2024-01-01T10:00:00Z"}
			},
			"db": {
				"backup": {"success": false, "error": "repo locked", "time": "2024-01-02T10:00:00Z"}
			}
		}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := readResticProfileStatusFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "db" has the later timestamp, so it should win.
	if status.LastStatus != "error" {
		t.Errorf("expected most recent profile (db, error) to win, got status=%q", status.LastStatus)
	}
	if status.ErrorMessage != "repo locked" {
		t.Errorf("expected error message %q, got %q", "repo locked", status.ErrorMessage)
	}
	if status.Source != "status_file" {
		t.Errorf("expected source=status_file, got %q", status.Source)
	}
}

func TestReadResticProfileStatusFile_NoBackupEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")
	if err := os.WriteFile(path, []byte(`{"profiles": {"files": {}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := readResticProfileStatusFile(path); err == nil {
		t.Error("expected an error when no profile has a backup entry")
	}
}

func TestReadResticProfileStatusFile_MissingFile(t *testing.T) {
	if _, err := readResticProfileStatusFile("/nonexistent/status.json"); err == nil {
		t.Error("expected an error for a missing status file")
	}
}

func TestListResticProfiles_Nominal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resticprofile.yaml")
	content := `version: "1"

global:
  status-file: /home/user/restic-backups/backup-status.json

files:
  backup:
    source:
      - /home/user/data

db:
  backup:
    source:
      - /home/user/restic-backups/mydb.sql
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	profiles, err := ListResticProfiles(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"db", "files"}
	if len(profiles) != len(want) || profiles[0] != want[0] || profiles[1] != want[1] {
		t.Errorf("expected profiles %v (sorted, reserved keys excluded), got %v", want, profiles)
	}
}

func TestListResticProfiles_Unconfigured(t *testing.T) {
	profiles, err := ListResticProfiles("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profiles != nil {
		t.Errorf("expected nil profiles when path is unconfigured, got %v", profiles)
	}
}

func TestListResticProfiles_MissingFile(t *testing.T) {
	profiles, err := ListResticProfiles("/nonexistent/resticprofile.yaml")
	if err != nil {
		t.Fatalf("expected a missing file to be tolerated, got error: %v", err)
	}
	if profiles != nil {
		t.Errorf("expected nil profiles for a missing file, got %v", profiles)
	}
}

func TestListResticGroups_Nominal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resticprofile.yaml")
	content := `version: "1"

global:
  status-file: /home/user/restic-backups/backup-status.json

groups:
  full-backup:
    - files
    - db
  files-only:
    - files

files:
  backup:
    source:
      - /home/user/data

db:
  backup:
    source:
      - /home/user/restic-backups/mydb.sql
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	groups, err := ListResticGroups(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"files-only", "full-backup"}
	if len(groups) != len(want) || groups[0] != want[0] || groups[1] != want[1] {
		t.Errorf("expected groups %v (sorted), got %v", want, groups)
	}
}

func TestListResticGroups_NoGroupsSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resticprofile.yaml")
	content := `version: "1"

files:
  backup:
    source:
      - /home/user/data
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	groups, err := ListResticGroups(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected no groups, got %v", groups)
	}
}

func TestListResticGroups_Unconfigured(t *testing.T) {
	groups, err := ListResticGroups("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if groups != nil {
		t.Errorf("expected nil groups when path is unconfigured, got %v", groups)
	}
}

func TestListResticGroups_MissingFile(t *testing.T) {
	groups, err := ListResticGroups("/nonexistent/resticprofile.yaml")
	if err != nil {
		t.Fatalf("expected a missing file to be tolerated, got error: %v", err)
	}
	if groups != nil {
		t.Errorf("expected nil groups for a missing file, got %v", groups)
	}
}

func TestParseResticProgressLine_Status(t *testing.T) {
	line := `{"message_type":"status","percent_done":0.5,"total_files":100,"files_done":50,"total_bytes":2000,"bytes_done":1000,"seconds_elapsed":10,"seconds_remaining":10}`
	var summary ResticBackupSummary
	ev, recognized := parseResticProgressLine(line, &summary)
	if !recognized {
		t.Fatal("expected a status line to be recognized")
	}
	if ev == nil {
		t.Fatal("expected a non-nil progress event")
	}
	if ev.Phase != "backup" {
		t.Errorf("expected phase=backup, got %q", ev.Phase)
	}
	if ev.PercentDone != 50 {
		t.Errorf("expected percent_done=50, got %v", ev.PercentDone)
	}
	if ev.FilesDone != 50 || ev.FilesTotal != 100 {
		t.Errorf("unexpected files done/total: %d/%d", ev.FilesDone, ev.FilesTotal)
	}
	if ev.ETASeconds != 10 {
		t.Errorf("expected eta_seconds=10, got %d", ev.ETASeconds)
	}
}

func TestParseResticProgressLine_Summary(t *testing.T) {
	line := `{"message_type":"summary","files_new":3,"files_changed":2,"data_added":4096,"snapshot_id":"abc123"}`
	var summary ResticBackupSummary
	ev, recognized := parseResticProgressLine(line, &summary)
	if !recognized {
		t.Fatal("expected a summary line to be recognized")
	}
	if ev == nil || ev.Phase != "summary" {
		t.Fatalf("expected phase=summary event, got %+v", ev)
	}
	if summary.FilesNew == nil || *summary.FilesNew != 3 {
		t.Errorf("expected FilesNew=3, got %v", summary.FilesNew)
	}
	if summary.FilesChanged == nil || *summary.FilesChanged != 2 {
		t.Errorf("expected FilesChanged=2, got %v", summary.FilesChanged)
	}
	if summary.BytesAdded == nil || *summary.BytesAdded != 4096 {
		t.Errorf("expected BytesAdded=4096, got %v", summary.BytesAdded)
	}
	if summary.SnapshotID != "abc123" {
		t.Errorf("expected SnapshotID=abc123, got %q", summary.SnapshotID)
	}
}

func TestParseResticProgressLine_PlainTextFallsThrough(t *testing.T) {
	var summary ResticBackupSummary
	ev, recognized := parseResticProgressLine("some plain log line", &summary)
	if recognized {
		t.Error("expected a non-JSON line to not be recognized as a progress/summary line")
	}
	if ev != nil {
		t.Error("expected a nil event for a non-JSON line")
	}
}

func TestParseResticProgressLine_UnknownMessageType(t *testing.T) {
	var summary ResticBackupSummary
	ev, recognized := parseResticProgressLine(`{"message_type":"verbose_status"}`, &summary)
	if recognized {
		t.Error("expected an unhandled message_type to fall through as unrecognized (streamed as text)")
	}
	if ev != nil {
		t.Error("expected a nil event for an unhandled message_type")
	}
}

func TestHasAllowedResticEnvPrefix(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"RESTIC_PASSWORD", true},
		{"RESTIC_REPOSITORY", true},
		{"OS_USERNAME", true},
		{"SWIFT_API_KEY", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"PATH", false},
		{"HOME", false},
		{"SHELL", false},
	}
	for _, tt := range tests {
		if got := hasAllowedResticEnvPrefix(tt.key); got != tt.want {
			t.Errorf("hasAllowedResticEnvPrefix(%q) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestLoadResticEnv_FiltersToAllowlistAndNeverLeaksOthers(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "resticconf")
	content := `export RESTIC_REPOSITORY=/tmp/repo
export RESTIC_PASSWORD=supersecret
export SOME_UNRELATED_SHELL_VAR=should-not-leak
`
	if err := os.WriteFile(confPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	env, err := loadResticEnv(context.Background(), confPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sawRepository, sawPassword bool
	for _, e := range env {
		if strings.HasPrefix(e, "RESTIC_REPOSITORY=") {
			sawRepository = true
		}
		if strings.HasPrefix(e, "RESTIC_PASSWORD=") {
			sawPassword = true
		}
		if strings.HasPrefix(e, "SOME_UNRELATED_SHELL_VAR=") {
			t.Errorf("unrelated shell var leaked through the allowlist: %q", e)
		}
	}
	if !sawRepository || !sawPassword {
		t.Errorf("expected RESTIC_REPOSITORY and RESTIC_PASSWORD to pass the allowlist, got %v", env)
	}
}

func TestLoadResticEnv_MissingConfPath(t *testing.T) {
	if _, err := loadResticEnv(context.Background(), ""); err == nil {
		t.Error("expected an error when restic_conf_path is not configured")
	}
	if _, err := loadResticEnv(context.Background(), "/nonexistent/resticconf"); err == nil {
		t.Error("expected an error when resticconf does not exist")
	}
}

func TestRunResticBackupWithProgress_MissingScript(t *testing.T) {
	cfg := &config.Config{ResticRunScriptPath: ""}
	summary, err := RunResticBackupWithProgress(context.Background(), cfg, "", nil)
	if err == nil {
		t.Fatal("expected an error when restic_run_script_path is not configured")
	}
	if summary != nil {
		t.Error("expected a nil summary when the script path is not configured")
	}
}

// TestRunResticBackupWithProgress_StreamsAndRedacts exercises the full path with a
// fake run_backup.sh script that emits a JSON progress line, a JSON summary line,
// and a plain text line containing a fake credential — verifying progress/summary
// parsing and that the credential-looking line is redacted before being streamed.
func TestRunResticBackupWithProgress_StreamsAndRedacts(t *testing.T) {
	dir := t.TempDir()

	confPath := filepath.Join(dir, "resticconf")
	if err := os.WriteFile(confPath, []byte("export RESTIC_REPOSITORY=/tmp/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(dir, "run_backup.sh")
	script := `#!/bin/bash
echo '{"message_type":"status","percent_done":0.25,"total_files":10,"files_done":2}'
echo "password: supersecret-value"
echo '{"message_type":"summary","files_new":1,"files_changed":0,"data_added":123,"snapshot_id":"deadbeef"}'
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		ResticConfPath:                 confPath,
		ResticRunScriptPath:            scriptPath,
		ResticEnableProgress:           true,
		ResticProgressFPS:              0.1,
		ResticBackupIdleTimeoutMinutes: 1,
	}

	var chunks []string
	_, err := RunResticBackupWithProgress(context.Background(), cfg, "files", func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	joined := strings.Join(chunks, "\n")
	if strings.Contains(joined, "supersecret-value") {
		t.Errorf("credential-looking value leaked into streamed output: %q", joined)
	}
	if !strings.Contains(joined, "[REDACTED]") {
		t.Errorf("expected the password line to be redacted, got %q", joined)
	}
	if !strings.Contains(joined, `"phase":"backup"`) {
		t.Errorf("expected a streamed backup progress event, got %q", joined)
	}
	if !strings.Contains(joined, `"phase":"summary"`) {
		t.Errorf("expected a streamed summary event, got %q", joined)
	}
}

func TestRunResticBackupWithProgress_ReportsScriptFailure(t *testing.T) {
	dir := t.TempDir()

	confPath := filepath.Join(dir, "resticconf")
	if err := os.WriteFile(confPath, []byte("export RESTIC_REPOSITORY=/tmp/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(dir, "run_backup.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho 'boom'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		ResticConfPath:                 confPath,
		ResticRunScriptPath:            scriptPath,
		ResticBackupIdleTimeoutMinutes: 1,
	}

	summary, err := RunResticBackupWithProgress(context.Background(), cfg, "", nil)
	if err == nil {
		t.Fatal("expected an error for a script exiting non-zero")
	}
	if summary == nil {
		t.Fatal("expected a summary to still be returned on failure")
	}
	if summary.Status != "error" {
		t.Errorf("expected summary.Status=error, got %q", summary.Status)
	}
	if summary.ErrorMessage == "" {
		t.Error("expected a non-empty ErrorMessage")
	}
}

func TestResticBackupSummary_DurationIsPopulated(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "resticconf")
	if err := os.WriteFile(confPath, []byte("export RESTIC_REPOSITORY=/tmp/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "run_backup.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\nsleep 0.2\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		ResticConfPath:                 confPath,
		ResticRunScriptPath:            scriptPath,
		ResticBackupIdleTimeoutMinutes: 1,
	}
	summary, err := RunResticBackupWithProgress(context.Background(), cfg, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.FinishedAt.Before(summary.StartedAt) {
		t.Errorf("FinishedAt (%v) should not be before StartedAt (%v)", summary.FinishedAt, summary.StartedAt)
	}
	if summary.DurationSec < 0 {
		t.Errorf("expected non-negative DurationSec, got %d", summary.DurationSec)
	}
}

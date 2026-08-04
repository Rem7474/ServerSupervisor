package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/security"
	"gopkg.in/yaml.v3"
)

// resticEnvAllowedPrefixes is the allowlist of environment variable name
// prefixes read out of resticconf and passed through to the restic/
// run_backup.sh subprocess. Anything else sourced from resticconf (shell
// helper vars, PS1, etc.) is dropped rather than forwarded wholesale.
var resticEnvAllowedPrefixes = []string{
	"RESTIC_", "OS_", "SWIFT_", "ST_", "B2_", "AWS_", "AZURE_", "GOOGLE_", "RCLONE_",
}

// ResticStatus is the periodic, passive snapshot of Restic's state included in
// the agent's regular report. Never contains resticconf content or resolved
// credential values — only facts about the last backup run and repo.
type ResticStatus struct {
	Installed     bool       `json:"installed"`
	LastRunAt     *time.Time `json:"last_run_at,omitempty"`
	LastStatus    string     `json:"last_status,omitempty"` // ok|error
	DurationSec   *int       `json:"duration_sec,omitempty"`
	FilesNew      *int       `json:"files_new,omitempty"`
	FilesChanged  *int       `json:"files_changed,omitempty"`
	BytesAdded    *int64     `json:"bytes_added,omitempty"`
	SnapshotID    string     `json:"snapshot_id,omitempty"`
	RepoSizeBytes *int64     `json:"repo_size_bytes,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	Source        string     `json:"source"` // status_file | restic_commands | unavailable
}

// ResticBackupSummary is the structured, final result of a run_backup action.
// It becomes the sole content of CommandResult.Output for that action — the
// human-readable log/progress was already delivered live via streamed chunks,
// so the terminal output is a compact, parseable summary instead of a
// duplicate of the log text.
type ResticBackupSummary struct {
	Status        string    `json:"status"` // ok|error
	Profile       string    `json:"profile,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	DurationSec   int       `json:"duration_sec"`
	FilesNew      *int      `json:"files_new,omitempty"`
	FilesChanged  *int      `json:"files_changed,omitempty"`
	BytesAdded    *int64    `json:"bytes_added,omitempty"`
	SnapshotID    string    `json:"snapshot_id,omitempty"`
	RepoSizeBytes *int64    `json:"repo_size_bytes,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
}

// ResticProgressEvent is a compact, structured representation of one restic
// --json progress line, streamed live to the server while a backup runs.
type ResticProgressEvent struct {
	Phase          string  `json:"phase,omitempty"` // backup|summary
	PercentDone    float64 `json:"percent_done,omitempty"`
	FilesDone      int64   `json:"files_done,omitempty"`
	FilesTotal     int64   `json:"files_total,omitempty"`
	BytesDone      int64   `json:"bytes_done,omitempty"`
	BytesTotal     int64   `json:"bytes_total,omitempty"`
	SecondsElapsed int64   `json:"seconds_elapsed,omitempty"`
	ETASeconds     int64   `json:"eta_seconds,omitempty"`
}

const resticCommandTimeout = 20 * time.Second

// CollectResticStatus reports the current Restic state. It prefers a
// resticprofile status-file (richer, cheap to read) and falls back to
// `restic snapshots`/`stats` when no status file is configured or readable.
// Never returns an error that would abort the caller's report cycle — an
// absent/misconfigured toolkit is reported as Installed:false, not a failure.
func CollectResticStatus(ctx context.Context, cfg *config.Config) (*ResticStatus, error) {
	bin := cfg.ResticBin
	if bin == "" {
		bin = "restic"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return &ResticStatus{Installed: false, Source: "unavailable"}, nil
	}

	if cfg.ResticStatusFilePath != "" {
		if status, err := readResticProfileStatusFile(cfg.ResticStatusFilePath); err == nil {
			status.Installed = true
			return status, nil
		}
	}

	return collectResticStatusFromCommands(ctx, cfg, bin), nil
}

type resticProfileStatusFile struct {
	Profiles map[string]struct {
		Backup *struct {
			Success bool      `json:"success"`
			Error   string    `json:"error,omitempty"`
			Time    time.Time `json:"time"`
		} `json:"backup,omitempty"`
	} `json:"profiles"`
}

// readResticProfileStatusFile parses resticprofile's JSON status-file and
// returns the most recently updated profile's backup result.
func readResticProfileStatusFile(path string) (*ResticStatus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var parsed resticProfileStatusFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}

	var latest *ResticStatus
	var latestTime time.Time
	for _, p := range parsed.Profiles {
		if p.Backup == nil || !p.Backup.Time.After(latestTime) {
			continue
		}
		latestTime = p.Backup.Time
		t := p.Backup.Time
		st := &ResticStatus{Source: "status_file", LastRunAt: &t}
		if p.Backup.Success {
			st.LastStatus = "ok"
		} else {
			st.LastStatus = "error"
			st.ErrorMessage = p.Backup.Error
		}
		latest = st
	}
	if latest == nil {
		return nil, fmt.Errorf("no backup entries in status file")
	}
	return latest, nil
}

type resticSnapshot struct {
	Time    time.Time `json:"time"`
	ShortID string    `json:"short_id"`
}

type resticStats struct {
	TotalSize int64 `json:"total_size"`
}

// collectResticStatusFromCommands falls back to `restic snapshots --json`
// and `restic stats --json` when no status-file is available. Never parses
// human-readable restic output — only its --json forms.
func collectResticStatusFromCommands(ctx context.Context, cfg *config.Config, bin string) *ResticStatus {
	status := &ResticStatus{Installed: true, Source: "restic_commands"}

	env, err := loadResticEnv(ctx, cfg.ResticConfPath)
	if err != nil {
		status.Source = "unavailable"
		status.ErrorMessage = "resticconf not readable"
		return status
	}

	snapCtx, cancel := context.WithTimeout(ctx, resticCommandTimeout)
	defer cancel()
	snapCmd := exec.CommandContext(snapCtx, bin, "snapshots", "--json", "--latest", "5")
	snapCmd.Env = append(os.Environ(), env...)
	if out, snapErr := snapCmd.Output(); snapErr == nil {
		var snaps []resticSnapshot
		if json.Unmarshal(out, &snaps) == nil && len(snaps) > 0 {
			latest := snaps[len(snaps)-1]
			t := latest.Time
			status.SnapshotID = latest.ShortID
			status.LastRunAt = &t
			status.LastStatus = "ok"
		}
	} else {
		status.ErrorMessage = "restic snapshots failed"
	}

	statsCtx, statsCancel := context.WithTimeout(ctx, resticCommandTimeout)
	defer statsCancel()
	statsCmd := exec.CommandContext(statsCtx, bin, "stats", "--json")
	statsCmd.Env = append(os.Environ(), env...)
	if out, statsErr := statsCmd.Output(); statsErr == nil {
		var stats resticStats
		if json.Unmarshal(out, &stats) == nil {
			size := stats.TotalSize
			status.RepoSizeBytes = &size
		}
	}

	return status
}

// reservedResticProfileKeys are resticprofile.yaml's non-profile top-level
// keys (see https://creativeprojects.github.io/resticprofile/configuration/)
// — never returned as a profile name.
var reservedResticProfileKeys = map[string]bool{
	"version": true, "global": true, "includes": true, "groups": true,
}

// ListResticProfiles reads resticprofile.yaml and returns its top-level
// profile names, sorted. That file holds only profile definitions (sources,
// excludes, schedule hints) — repository/backend credentials live in
// resticconf, never here — so parsing it locally is safe; only the
// resulting profile *names* are ever reported to the server, never the file
// content itself. Returns (nil, nil) when unconfigured or the file is
// absent, consistent with the rest of this collector's tolerant-of-missing-
// setup behavior.
func ListResticProfiles(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	profiles := make([]string, 0, len(doc))
	for key := range doc {
		if reservedResticProfileKeys[key] {
			continue
		}
		profiles = append(profiles, key)
	}
	sort.Strings(profiles)
	return profiles, nil
}

// ListResticGroups reads resticprofile.yaml's top-level "groups" section and
// returns the group names defined there, sorted. A group bundles several
// profiles to run together and is resolved by resticprofile itself when
// passed to `resticprofile --name <group>` — from run_backup.sh's point of
// view a group name and a profile name are interchangeable, so this is a
// second, separate discovery pass rather than a variant of
// ListResticProfiles: unlike a profile, a group is a *value* nested one level
// under the reserved "groups" key, not a top-level key itself.
func ListResticGroups(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Groups map[string]any `yaml:"groups"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	groups := make([]string, 0, len(doc.Groups))
	for key := range doc.Groups {
		groups = append(groups, key)
	}
	sort.Strings(groups)
	return groups, nil
}

// RunResticBackupWithProgress runs the configured run_backup.sh script for the
// given resticprofile profile (empty = script default), streaming progress to
// chunkCB. Uses an idle-timeout watchdog (via runCommandWithStreaming, shared
// with the apt module) rather than an absolute deadline — a backup that keeps
// progressing can legitimately run far longer than a fixed cap.
func RunResticBackupWithProgress(ctx context.Context, cfg *config.Config, profile string, chunkCB func(string)) (*ResticBackupSummary, error) {
	if cfg.ResticRunScriptPath == "" {
		return nil, fmt.Errorf("restic_run_script_path is not configured")
	}
	if _, err := os.Stat(cfg.ResticRunScriptPath); err != nil {
		return nil, fmt.Errorf("run_backup script not found: %w", err)
	}

	env, err := loadResticEnv(ctx, cfg.ResticConfPath)
	if err != nil {
		return nil, err
	}

	var args []string
	if profile != "" {
		args = append(args, profile)
	}
	cmd := exec.Command(cfg.ResticRunScriptPath, args...) //nolint:gosec // path is agent-local config, never server-supplied
	cmd.Env = append(os.Environ(), env...)
	if cfg.ResticEnableProgress {
		fps := cfg.ResticProgressFPS
		if fps <= 0 {
			fps = 0.1
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("RESTIC_PROGRESS_FPS=%g", fps))
	}

	summary := &ResticBackupSummary{Profile: profile, StartedAt: time.Now()}

	wrapped := func(chunk string) {
		if chunkCB == nil {
			// Still parse (to fill in summary.FilesNew/SnapshotID/etc. from the
			// "summary" line), just don't stream anything.
			for _, line := range strings.Split(chunk, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					parseResticProgressLine(line, summary)
				}
			}
			return
		}
		for _, line := range strings.Split(chunk, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if ev, recognized := parseResticProgressLine(line, summary); recognized {
				if ev != nil {
					if b, marshalErr := json.Marshal(ev); marshalErr == nil {
						chunkCB(string(b))
					}
				}
				continue
			}
			chunkCB(security.FilterYAML(line))
		}
	}

	idleTimeout := time.Duration(cfg.ResticBackupIdleTimeoutMinutes) * time.Minute
	if idleTimeout <= 0 {
		idleTimeout = 20 * time.Minute
	}

	_, runErr := runCommandWithStreaming(cmd, wrapped, idleTimeout)
	summary.FinishedAt = time.Now()
	summary.DurationSec = int(summary.FinishedAt.Sub(summary.StartedAt).Seconds())

	if runErr != nil {
		summary.Status = "error"
		summary.ErrorMessage = runErr.Error()
		return summary, runErr
	}
	if summary.Status == "" {
		summary.Status = "ok"
	}
	return summary, nil
}

type resticJSONLine struct {
	MessageType      string  `json:"message_type"`
	PercentDone      float64 `json:"percent_done"`
	TotalFiles       int64   `json:"total_files"`
	FilesDone        int64   `json:"files_done"`
	TotalBytes       int64   `json:"total_bytes"`
	BytesDone        int64   `json:"bytes_done"`
	SecondsElapsed   int64   `json:"seconds_elapsed"`
	SecondsRemaining int64   `json:"seconds_remaining"`
	FilesNew         int     `json:"files_new"`
	FilesChanged     int     `json:"files_changed"`
	DataAdded        int64   `json:"data_added"`
	SnapshotID       string  `json:"snapshot_id"`
}

// parseResticProgressLine recognizes restic's --json "status" (periodic
// progress) and "summary" (final result) message types. Returns
// recognized=false for anything else (plain text, or JSON of an unhandled
// message_type) so the caller falls back to streaming it as a redacted text
// line rather than dropping it.
func parseResticProgressLine(line string, summary *ResticBackupSummary) (ev *ResticProgressEvent, recognized bool) {
	if !strings.HasPrefix(line, "{") {
		return nil, false
	}
	var msg resticJSONLine
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil, false
	}

	switch msg.MessageType {
	case "status":
		return &ResticProgressEvent{
			Phase:          "backup",
			PercentDone:    msg.PercentDone * 100,
			FilesDone:      msg.FilesDone,
			FilesTotal:     msg.TotalFiles,
			BytesDone:      msg.BytesDone,
			BytesTotal:     msg.TotalBytes,
			SecondsElapsed: msg.SecondsElapsed,
			ETASeconds:     msg.SecondsRemaining,
		}, true
	case "summary":
		filesNew := msg.FilesNew
		filesChanged := msg.FilesChanged
		bytesAdded := msg.DataAdded
		summary.FilesNew = &filesNew
		summary.FilesChanged = &filesChanged
		summary.BytesAdded = &bytesAdded
		summary.SnapshotID = msg.SnapshotID
		return &ResticProgressEvent{Phase: "summary", PercentDone: 100}, true
	default:
		return nil, false
	}
}

// loadResticEnv sources resticconf in a subshell and returns only the
// environment variables whose names match resticEnvAllowedPrefixes — never
// logs or returns anything else from that shell's environment, and never
// returns the file's content itself.
func loadResticEnv(ctx context.Context, confPath string) ([]string, error) {
	if confPath == "" {
		return nil, fmt.Errorf("restic_conf_path is not configured")
	}
	if _, err := os.Stat(confPath); err != nil {
		return nil, fmt.Errorf("resticconf not found: %w", err)
	}

	sourceCtx, cancel := context.WithTimeout(ctx, resticCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(sourceCtx, "bash", "-lc", fmt.Sprintf("set -a; source %q; env", confPath))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to source resticconf: %w", err)
	}

	var filtered []string
	for _, line := range strings.Split(string(out), "\n") {
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if hasAllowedResticEnvPrefix(key) {
			filtered = append(filtered, line)
		}
	}
	return filtered, nil
}

func hasAllowedResticEnvPrefix(key string) bool {
	for _, p := range resticEnvAllowedPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
}

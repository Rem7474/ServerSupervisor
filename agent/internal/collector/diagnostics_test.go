package collector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/serversupervisor/agent/internal/config"
)

func hasIssue(issues []DiagnosticIssue, collector string) bool {
	for _, i := range issues {
		if i.Collector == collector {
			return true
		}
	}
	return false
}

func TestCheckConfig_AllDisabled_NoIssues(t *testing.T) {
	cfg := &config.Config{}
	issues := CheckConfig(cfg)
	if len(issues) != 0 {
		t.Errorf("expected no issues with every collector disabled, got %v", issues)
	}
}

func TestCheckConfig_Restic_MissingConfPath_Error(t *testing.T) {
	cfg := &config.Config{CollectRestic: true, ResticBin: "/bin/true"}
	issues := CheckConfig(cfg)
	found := false
	for _, i := range issues {
		if i.Collector == "restic" && i.Severity == DiagnosticError {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a restic error for unconfigured restic_conf_path, got %v", issues)
	}
}

func TestCheckConfig_Restic_ConfPathNotFound_Error(t *testing.T) {
	cfg := &config.Config{
		CollectRestic:  true,
		ResticBin:      "/bin/true",
		ResticConfPath: "/nonexistent/resticconf",
	}
	issues := CheckConfig(cfg)
	found := false
	for _, i := range issues {
		if i.Collector == "restic" && i.Severity == DiagnosticError {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a restic error for a resticconf path that doesn't exist, got %v", issues)
	}
}

func TestCheckConfig_Restic_ValidMinimalConfig_NoConfPathIssue(t *testing.T) {
	if _, err := os.Stat("/bin/true"); err != nil {
		t.Skip("/bin/true not present on this system")
	}
	dir := t.TempDir()
	confPath := filepath.Join(dir, "resticconf")
	if err := os.WriteFile(confPath, []byte("export RESTIC_REPOSITORY=/tmp/repo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		CollectRestic:  true,
		ResticBin:      "/bin/true",
		ResticConfPath: confPath,
	}
	issues := CheckConfig(cfg)
	for _, i := range issues {
		if i.Collector == "restic" && i.Severity == DiagnosticError {
			t.Errorf("expected no restic error with a valid bin + resticconf, got %v", i)
		}
	}
}

func TestCheckConfig_Restic_BinNotFound_Error(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "resticconf")
	if err := os.WriteFile(confPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		CollectRestic:  true,
		ResticBin:      "/nonexistent/bin/restic",
		ResticConfPath: confPath,
	}
	issues := CheckConfig(cfg)
	found := false
	for _, i := range issues {
		if i.Collector == "restic" && i.Severity == DiagnosticError {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a restic error for a restic_bin that resolves nowhere, got %v", issues)
	}
}

func TestCheckConfig_Restic_RunScriptNotExecutable_Warning(t *testing.T) {
	if _, err := os.Stat("/bin/true"); err != nil {
		t.Skip("/bin/true not present on this system")
	}
	dir := t.TempDir()
	confPath := filepath.Join(dir, "resticconf")
	if err := os.WriteFile(confPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	scriptPath := filepath.Join(dir, "run_backup.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\n"), 0o600); err != nil { // no exec bit
		t.Fatal(err)
	}
	cfg := &config.Config{
		CollectRestic:       true,
		ResticBin:           "/bin/true",
		ResticConfPath:      confPath,
		ResticRunScriptPath: scriptPath,
	}
	issues := CheckConfig(cfg)
	found := false
	for _, i := range issues {
		if i.Collector == "restic" && i.Severity == DiagnosticWarning {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a restic warning for a non-executable run_backup.sh, got %v", issues)
	}
}

func TestCheckConfig_Restic_ProfileConfigMissing_Warning(t *testing.T) {
	if _, err := os.Stat("/bin/true"); err != nil {
		t.Skip("/bin/true not present on this system")
	}
	dir := t.TempDir()
	confPath := filepath.Join(dir, "resticconf")
	if err := os.WriteFile(confPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		CollectRestic:           true,
		ResticBin:               "/bin/true",
		ResticConfPath:          confPath,
		ResticProfileConfigPath: "/nonexistent/resticprofile.yaml",
	}
	issues := CheckConfig(cfg)
	found := false
	for _, i := range issues {
		if i.Collector == "restic" && i.Severity == DiagnosticWarning {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a restic warning for a configured-but-missing resticprofile.yaml, got %v", issues)
	}
}

func TestCheckConfig_WebLogs_NoMatchingFiles_Warning(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		CollectWebLogs:  true,
		WebLogsLogPaths: []string{filepath.Join(dir, "nothing-here-*.log")},
	}
	issues := CheckConfig(cfg)
	if !hasIssue(issues, "web_logs") {
		t.Errorf("expected a web_logs warning when no log file matches, got %v", issues)
	}
}

func TestCheckConfig_WebLogs_MatchingFile_NoGlobIssue(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")
	if err := os.WriteFile(logPath, []byte("127.0.0.1 - - [GET /]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		CollectWebLogs:  true,
		WebLogsLogPaths: []string{logPath},
	}
	issues := CheckConfig(cfg)
	if hasIssue(issues, "web_logs") {
		t.Errorf("expected no web_logs issue when the configured path matches a real file, got %v", issues)
	}
}

func TestCheckConfig_CrowdSec_WithoutWebLogs_Warning(t *testing.T) {
	cfg := &config.Config{
		CollectCrowdSecCorrelation: true,
		CollectWebLogs:             false,
	}
	issues := CheckConfig(cfg)
	if !hasIssue(issues, "crowdsec") {
		t.Errorf("expected a crowdsec warning when collect_web_logs is off, got %v", issues)
	}
}

func TestCheckConfig_CrowdSec_WithWebLogsAndMatchingFile_NoWarning(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "access.log")
	if err := os.WriteFile(logPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		CollectCrowdSecCorrelation: true,
		CollectWebLogs:             true,
		WebLogsLogPaths:            []string{logPath},
	}
	issues := CheckConfig(cfg)
	if hasIssue(issues, "crowdsec") {
		t.Errorf("expected no crowdsec issue when collect_web_logs is on, got %v", issues)
	}
}

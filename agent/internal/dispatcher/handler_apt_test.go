package dispatcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/sender"
)

// fakeAgentServer records every /api/agent/command/result body posted to it,
// so a test can inspect exactly what a handler reported without a real
// ServerSupervisor server.
func fakeAgentServer(t *testing.T) (*httptest.Server, func() []map[string]any) {
	t.Helper()
	var mu sync.Mutex
	var results []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agent/command/result" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			results = append(results, body)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	return srv, func() []map[string]any {
		mu.Lock()
		defer mu.Unlock()
		out := make([]map[string]any, len(results))
		copy(out, results)
		return out
	}
}

// lastNonRunningResult returns the last posted command result whose status
// isn't the intermediate "running" report — i.e. the terminal one.
func lastNonRunningResult(t *testing.T, results []map[string]any) map[string]any {
	t.Helper()
	for i := len(results) - 1; i >= 0; i-- {
		if results[i]["status"] != "running" {
			return results[i]
		}
	}
	t.Fatal("expected a terminal (non-\"running\") command result to be reported")
	return nil
}

// TestHandleApt_RunUU_BundlesFreshAptStatus is the regression guard for the
// pending-package-count staleness bug: a manual run_uu can itself install
// pending upgrades, so its terminal report must carry a fresh apt_status
// (not just unattended_upgrades) the same way the default apt branch already
// does for update/upgrade/full-upgrade/autoremove — without it, the "Paquets
// en attente" KPI never reflects what run_uu just did (see reportRunUUTerminal's
// own doc comment for why the periodic-report fallback doesn't catch this
// either). apt-get/dpkg are real (read-only: `apt-get upgrade --simulate`,
// `dpkg -s`) — no unattended-upgrade binary exists in this test environment,
// so the actual upgrade attempt itself fails harmlessly; reportRunUUTerminal
// bundles the fresh status regardless of that outcome.
func TestHandleApt_RunUU_BundlesFreshAptStatus(t *testing.T) {
	srv, results := fakeAgentServer(t)
	s := sender.New(&config.Config{ServerURL: srv.URL, APIKey: "test-key"})
	cmd := sender.PendingCommand{ID: "cmd-run-uu", Module: "apt", Action: "run_uu"}

	handleApt(context.Background(), nil, s, cmd)

	terminal := lastNonRunningResult(t, results())
	if terminal["apt_status"] == nil {
		t.Error("expected run_uu's terminal report to bundle a fresh apt_status, got nil")
	}
	if terminal["unattended_upgrades"] == nil {
		t.Error("expected run_uu's terminal report to bundle a fresh unattended_upgrades snapshot, got nil")
	}
}

// install_uu/toggle_uu/configure_uu are deliberately not covered here the
// way run_uu is above: run_uu is safe to exercise for real because the
// unattended-upgrade binary doesn't exist in this test environment, so the
// actual upgrade attempt fails fast before doing anything — but
// install_uu runs a real `apt-get install`, and toggle_uu/configure_uu write
// real files under /etc/apt/apt.conf.d/. Exercising any of those three would
// mutate the test runner's actual system state, which no test in this suite
// should ever do.

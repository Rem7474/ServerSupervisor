package api

import (
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/scheduler"
	"github.com/serversupervisor/server/internal/testutil"
	"github.com/serversupervisor/server/internal/ws"
)

// apiKeyOpenUndocumentedRoutes lists the /api/v1 routes an API key may reach
// (within its owner's role and its scope) that the OpenAPI spec does not
// describe. Together with the spec and apiKeyForbiddenRoute it classifies every
// route: TestAPIKeyRouteClassification fails on a route that is in none of
// them, so a new endpoint is exposed to keys only by a deliberate choice —
// document it, refuse it in apiKeyForbiddenRoute, or add it here.
var apiKeyOpenUndocumentedRoutes = map[string]bool{
	"GET /api/v1/alert-rule-templates":                         true,
	"POST /api/v1/alert-rule-templates":                        true,
	"DELETE /api/v1/alert-rule-templates/:id":                  true,
	"GET /api/v1/alert-rule-templates/:id":                     true,
	"PATCH /api/v1/alert-rule-templates/:id":                   true,
	"POST /api/v1/alert-rule-templates/:id/apply":              true,
	"POST /api/v1/alert-rules":                                 true,
	"DELETE /api/v1/alert-rules/:id":                           true,
	"GET /api/v1/alert-rules/:id":                              true,
	"PATCH /api/v1/alert-rules/:id":                            true,
	"GET /api/v1/alert-rules/capabilities/agent":               true,
	"GET /api/v1/alert-rules/capabilities/docker":              true,
	"GET /api/v1/alert-rules/capabilities/proxmox":             true,
	"GET /api/v1/alert-rules/capabilities/synthetic":           true,
	"POST /api/v1/alert-rules/test":                            true,
	"POST /api/v1/alert-rules/test/logs":                       true,
	"POST /api/v1/alerts/incidents/:id/resolve":                true,
	"POST /api/v1/apt/command":                                 true,
	"GET /api/v1/audit/commands":                               true,
	"GET /api/v1/audit/logs":                                   true,
	"GET /api/v1/audit/logs/export":                            true,
	"GET /api/v1/audit/logs/host/:host_id":                     true,
	"GET /api/v1/audit/logs/me":                                true,
	"GET /api/v1/audit/logs/user/:username":                    true,
	"GET /api/v1/backup/runs/:runId":                           true,
	"GET /api/v1/commands/:id":                                 true,
	"POST /api/v1/commands/:id/cancel":                         true,
	"GET /api/v1/dashboard/init":                               true,
	"POST /api/v1/docker/command":                              true,
	"GET /api/v1/docker/compose":                               true,
	"GET /api/v1/host-groups":                                  true,
	"POST /api/v1/host-groups":                                 true,
	"DELETE /api/v1/host-groups/:id":                           true,
	"PUT /api/v1/host-groups/:id":                              true,
	"DELETE /api/v1/hosts/:id":                                 true,
	"PATCH /api/v1/hosts/:id":                                  true,
	"POST /api/v1/hosts/:id/agent/update":                      true,
	"GET /api/v1/hosts/:id/apt/unattended-upgrades":            true,
	"PUT /api/v1/hosts/:id/apt/unattended-upgrades":            true,
	"POST /api/v1/hosts/:id/apt/unattended-upgrades/install":   true,
	"POST /api/v1/hosts/:id/apt/unattended-upgrades/run-now":   true,
	"GET /api/v1/hosts/:id/apt/unattended-upgrades/runs":       true,
	"GET /api/v1/hosts/:id/backup":                             true,
	"GET /api/v1/hosts/:id/backup/groups":                      true,
	"GET /api/v1/hosts/:id/backup/profiles":                    true,
	"POST /api/v1/hosts/:id/backup/run":                        true,
	"GET /api/v1/hosts/:id/backup/runs":                        true,
	"GET /api/v1/hosts/:id/capabilities":                       true,
	"GET /api/v1/hosts/:id/commands/history":                   true,
	"GET /api/v1/hosts/:id/complete":                           true,
	"GET /api/v1/hosts/:id/compose-projects":                   true,
	"GET /api/v1/hosts/:id/custom-tasks":                       true,
	"POST /api/v1/hosts/:id/custom-tasks/:taskId/run":          true,
	"GET /api/v1/hosts/:id/dashboard":                          true,
	"GET /api/v1/hosts/:id/disk/health":                        true,
	"GET /api/v1/hosts/:id/disk/metrics":                       true,
	"GET /api/v1/hosts/:id/disk/metrics/aggregated":            true,
	"GET /api/v1/hosts/:id/disk/metrics/history":               true,
	"GET /api/v1/hosts/:id/exposure":                           true,
	"GET /api/v1/hosts/:id/maintenance-windows":                true,
	"GET /api/v1/hosts/:id/network/flows":                      true,
	"GET /api/v1/hosts/:id/network/flows/history":              true,
	"GET /api/v1/hosts/:id/network/flows/summary":              true,
	"GET /api/v1/hosts/:id/proxmox-candidates":                 true,
	"GET /api/v1/hosts/:id/proxmox-disks":                      true,
	"GET /api/v1/hosts/:id/proxmox-link":                       true,
	"GET /api/v1/hosts/:id/scheduled-tasks":                    true,
	"POST /api/v1/hosts/:id/scheduled-tasks":                   true,
	"GET /api/v1/hosts/:id/tasks-yaml":                         true,
	"GET /api/v1/hosts/:id/timeline":                           true,
	"POST /api/v1/hosts/discover":                              true,
	"DELETE /api/v1/maintenance-windows/:id":                   true,
	"POST /api/v1/maintenance-windows/global":                  true,
	"GET /api/v1/network":                                      true,
	"GET /api/v1/network/config":                               true,
	"PUT /api/v1/network/config":                               true,
	"GET /api/v1/network/ip-inventory":                         true,
	"GET /api/v1/network/topology":                             true,
	"GET /api/v1/notifications":                                true,
	"POST /api/v1/notifications/mark-read":                     true,
	"GET /api/v1/npm/proxy-hosts":                              true,
	"PATCH /api/v1/npm/proxy-hosts/:id":                        true,
	"PATCH /api/v1/npm/proxy-hosts/:id/npm-enabled":            true,
	"GET /api/v1/proxmox/backup-jobs":                          true,
	"GET /api/v1/proxmox/backup-runs":                          true,
	"POST /api/v1/proxmox/guests/:id/action":                   true,
	"GET /api/v1/proxmox/guests/:id/exposure":                  true,
	"GET /api/v1/proxmox/guests/:id/link":                      true,
	"GET /api/v1/proxmox/guests/:id/metrics":                   true,
	"GET /api/v1/proxmox/links":                                true,
	"POST /api/v1/proxmox/links":                               true,
	"DELETE /api/v1/proxmox/links/:id":                         true,
	"GET /api/v1/proxmox/links/:id":                            true,
	"PUT /api/v1/proxmox/links/:id":                            true,
	"GET /api/v1/proxmox/nodes/:id":                            true,
	"POST /api/v1/proxmox/nodes/:id/apt-refresh":               true,
	"GET /api/v1/proxmox/nodes/:id/cpu-temp/history":           true,
	"GET /api/v1/proxmox/nodes/:id/disks":                      true,
	"GET /api/v1/proxmox/nodes/:id/fan-rpm/history":            true,
	"GET /api/v1/proxmox/nodes/:id/guest-exposure":             true,
	"GET /api/v1/proxmox/nodes/:id/guest-networks":             true,
	"POST /api/v1/proxmox/nodes/:id/guests/:vmid/migrate":      true,
	"GET /api/v1/proxmox/nodes/:id/rrd":                        true,
	"PUT /api/v1/proxmox/nodes/:id/sensor-source":              true,
	"GET /api/v1/proxmox/nodes/:id/sensor-source/candidates":   true,
	"GET /api/v1/proxmox/nodes/:id/services":                   true,
	"POST /api/v1/proxmox/nodes/:id/services/:service/:action": true,
	"GET /api/v1/proxmox/nodes/:id/status":                     true,
	"GET /api/v1/proxmox/nodes/:id/syslog":                     true,
	"GET /api/v1/proxmox/nodes/:id/tasks":                      true,
	"GET /api/v1/proxmox/nodes/:id/tasks/:upid/log":            true,
	"GET /api/v1/proxmox/nodes/metrics":                        true,
	"GET /api/v1/proxmox/tasks":                                true,
	"GET /api/v1/release-trackers":                             true,
	"POST /api/v1/release-trackers":                            true,
	"DELETE /api/v1/release-trackers/:id":                      true,
	"GET /api/v1/release-trackers/:id":                         true,
	"PUT /api/v1/release-trackers/:id":                         true,
	"POST /api/v1/release-trackers/:id/check-now":              true,
	"GET /api/v1/release-trackers/:id/executions":              true,
	"POST /api/v1/release-trackers/:id/run":                    true,
	"GET /api/v1/release-trackers/:id/version-history":         true,
	"POST /api/v1/release-trackers/bulk":                       true,
	"GET /api/v1/release-trackers/pickable-containers":         true,
	"GET /api/v1/release-trackers/trackable-containers":        true,
	"GET /api/v1/runbooks":                                     true,
	"POST /api/v1/runbooks":                                    true,
	"DELETE /api/v1/runbooks/:id":                              true,
	"GET /api/v1/runbooks/:id":                                 true,
	"PATCH /api/v1/runbooks/:id":                               true,
	"GET /api/v1/runbooks/:id/executions":                      true,
	"GET /api/v1/runbooks/:id/executions/:execution_id":        true,
	"POST /api/v1/runbooks/:id/run":                            true,
	"GET /api/v1/scheduled-tasks":                              true,
	"DELETE /api/v1/scheduled-tasks/:id":                       true,
	"PUT /api/v1/scheduled-tasks/:id":                          true,
	"GET /api/v1/scheduled-tasks/:id/executions":               true,
	"POST /api/v1/scheduled-tasks/:id/run":                     true,
	"GET /api/v1/security/web-logs":                            true,
	"GET /api/v1/security/web-logs/domain/:domain":             true,
	"GET /api/v1/security/web-logs/ip/:ip":                     true,
	"DELETE /api/v1/security/web-logs/ip/:ip/decisions":        true,
	"POST /api/v1/security/web-logs/ip/:ip/decisions":          true,
	"GET /api/v1/security/web-logs/live":                       true,
	"GET /api/v1/security/web-logs/timeseries":                 true,
	"POST /api/v1/ssl/certificates":                            true,
	"DELETE /api/v1/ssl/certificates/:id":                      true,
	"PUT /api/v1/ssl/certificates/:id":                         true,
	"POST /api/v1/ssl/certificates/:id/check-now":              true,
	"GET /api/v1/ssl/certificates/:id/history":                 true,
	"POST /api/v1/system/journalctl":                           true,
	"POST /api/v1/system/processes":                            true,
	"POST /api/v1/system/service":                              true,
	"POST /api/v1/uptime/probes":                               true,
	"DELETE /api/v1/uptime/probes/:id":                         true,
	"PUT /api/v1/uptime/probes/:id":                            true,
	"POST /api/v1/uptime/probes/:id/check-now":                 true,
	"GET /api/v1/uptime/probes/:id/history/buckets":            true,
	"GET /api/v1/uptime/probes/:id/stats":                      true,
}

// TestAPIKeyRouteClassification walks every route SetupRouter registers under
// /api/v1 and requires each one to be documented in the OpenAPI spec, refused
// to API keys, or listed in apiKeyOpenUndocumentedRoutes — exactly one of them.
func TestAPIKeyRouteClassification(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	disp := dispatch.New(db)
	r, _, _, _, cleanup := SetupRouter(db, cfg, ws.NewNotificationHub(), events.NewBus(), scheduler.New(db, disp), disp)
	t.Cleanup(cleanup)

	documented, _ := documentedRoutes(t)
	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		if !strings.HasPrefix(ri.Path, "/api/v1/") {
			continue
		}
		route := ri.Method + " " + ri.Path
		registered[route] = true
		// WebSocket routes authenticate with WSTokenMiddleware (session
		// cookie or in-band token), never with an API key.
		if strings.HasPrefix(ri.Path, "/api/v1/ws/") {
			continue
		}
		_, isDocumented := documented[route]
		classes := 0
		for _, in := range []bool{isDocumented, apiKeyForbiddenRoute(ri.Method, ri.Path), apiKeyOpenUndocumentedRoutes[route]} {
			if in {
				classes++
			}
		}
		switch {
		case classes == 0:
			t.Errorf("%s is not classified for API keys: document it in openapi.json, refuse it in apiKeyForbiddenRoute, or add it to apiKeyOpenUndocumentedRoutes", route)
		case classes > 1:
			t.Errorf("%s is classified more than once (documented / refused to keys / open undocumented)", route)
		}
	}
	for route := range apiKeyOpenUndocumentedRoutes {
		if !registered[route] {
			t.Errorf("apiKeyOpenUndocumentedRoutes lists %s, which the router does not register", route)
		}
	}
}

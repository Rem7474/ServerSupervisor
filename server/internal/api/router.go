package api

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/cookies"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/networkview"
	"github.com/serversupervisor/server/internal/scheduler"
	aptsvc "github.com/serversupervisor/server/internal/services/apt"
	auditsvc "github.com/serversupervisor/server/internal/services/audit"
	dockersvc "github.com/serversupervisor/server/internal/services/docker"
	hostsvc "github.com/serversupervisor/server/internal/services/host"
	hostpermsvc "github.com/serversupervisor/server/internal/services/hostperm"
	networksvc "github.com/serversupervisor/server/internal/services/network"
	notifssvc "github.com/serversupervisor/server/internal/services/notifications"
	npmsvc "github.com/serversupervisor/server/internal/services/npm"
	pushsvc "github.com/serversupervisor/server/internal/services/push"
	scheduledtasksvc "github.com/serversupervisor/server/internal/services/scheduledtask"
	settingssvc "github.com/serversupervisor/server/internal/services/settings"
	sslsvc "github.com/serversupervisor/server/internal/services/ssl"
	uptimesvc "github.com/serversupervisor/server/internal/services/uptime"
	usersvc "github.com/serversupervisor/server/internal/services/user"
	weblogssvc "github.com/serversupervisor/server/internal/services/weblogs"
	"github.com/serversupervisor/server/internal/ws"
)

// SetupRouter wires all handlers and registers route groups.
// The caller is responsible for starting long-running poller services after this function returns.
// The returned cleanup func must be called on shutdown to stop background goroutines (rate limiters).
func SetupRouter(db *database.DB, cfg *config.Config, notifHub *ws.NotificationHub, sched *scheduler.TaskScheduler, dispatcher *dispatch.Dispatcher) (*gin.Engine, *handlers.ReleaseTrackerHandler, *handlers.ProxmoxHandler, *handlers.NPMHandler, func()) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestIDMiddleware())
	r.Use(RequestLogger())
	r.Use(SecurityHeadersMiddleware())
	r.Use(CORSMiddleware(cfg.BaseURL, cfg.AllowedOrigins))

	ipRateLimiter := NewIPRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst, cfg.TrustedProxyCIDRs)
	r.Use(RateLimiterMiddleware(ipRateLimiter))
	agentRateLimiter := NewIPRateLimiter(cfg.AgentRateLimitRPS, cfg.AgentRateLimitBurst, cfg.TrustedProxyCIDRs)
	// Stricter limiter for the unauthenticated public webhook receiver (5 req/s, burst 10).
	webhookRateLimiter := NewIPRateLimiter(5, 10, cfg.TrustedProxyCIDRs)

	// Instantiate handlers
	authH := handlers.NewAuthHandler(db, cfg, dispatcher)
	hostH := handlers.NewHostHandler(hostsvc.NewService(db, dispatcher, func() string {
		return handlers.ResolveLatestAgentVersion(cfg)
	}))
	wsH := ws.NewWSHandler(db, cfg, notifHub)
	agentH := handlers.NewAgentHandler(db, cfg, wsH.GetStreamHub(), notifHub)
	aptH := handlers.NewAptHandler(aptsvc.NewService(db, dispatcher), db)
	dockerH := handlers.NewDockerHandler(dockersvc.NewService(db, dispatcher), db)
	systemH := handlers.NewSystemHandler(db, cfg, dispatcher, wsH.GetStreamHub())
	networkH := handlers.NewNetworkHandler(networksvc.NewService(db, func(ctx context.Context) (*models.NetworkSnapshot, error) {
		return networkview.BuildSnapshot(ctx, db)
	}))
	auditH := handlers.NewAuditHandler(auditsvc.NewService(db))
	userH := handlers.NewUserHandler(usersvc.NewService(db))
	alertRulesH := handlers.NewAlertRulesHandler(db, cfg)
	settingsH := handlers.NewSettingsHandler(settingssvc.NewService(db, cfg, func() string {
		return handlers.ResolveLatestAgentVersion(cfg)
	}))
	notifH := handlers.NewNotificationsHandler(notifssvc.NewService(db, func(ctx context.Context, rule models.AlertRule, hostID string) (float64, bool) {
		return alerts.CurrentIncidentValue(ctx, db, rule, hostID)
	}))
	pushH := handlers.NewPushHandler(pushsvc.NewService(db))
	scheduledTaskH := handlers.NewScheduledTaskHandler(scheduledtasksvc.NewService(db, sched, dispatcher), db)
	gitWebhookH := handlers.NewGitWebhookHandler(db, cfg, dispatcher, notifHub)
	releaseTrackerH := handlers.NewReleaseTrackerHandler(db, cfg, dispatcher, notifHub)
	agentH.AddCompletionListener(gitWebhookH)
	agentH.AddCompletionListener(releaseTrackerH)

	proxmoxH := handlers.NewProxmoxHandler(db, cfg)
	hostPermH := handlers.NewHostPermissionHandler(hostpermsvc.NewService(db))
	uptimeH := handlers.NewUptimeHandler(uptimesvc.NewService(db))
	sslH := handlers.NewSSLHandler(sslsvc.NewService(db))
	webLogsH := handlers.NewWebLogsHandler(weblogssvc.NewService(db, dispatcher))
	npmH := handlers.NewNPMHandler(npmsvc.NewService(db))

	registerPublicRoutes(r, authH, db)
	registerWSRoutes(r, wsH, cfg)
	registerAgentRoutes(r, db, cfg, agentH, agentRateLimiter)

	v1 := r.Group("/api/v1")
	v1.Use(JWTMiddleware(cfg))
	v1.Use(cookies.CSRFMiddleware())
	registerAuthRoutes(v1, authH)
	registerWebLogsRoutes(v1, webLogsH)
	registerHostRoutes(v1, hostH, agentH, db)
	registerDockerRoutes(v1, dockerH, systemH, networkH, agentH)
	registerAPTRoutes(v1, aptH)
	registerAuditRoutes(v1, auditH)
	registerAlertRoutes(v1, alertRulesH)
	registerNotifRoutes(v1, notifH)
	registerPushRoutes(v1, pushH)
	registerSettingsRoutes(v1, settingsH)
	registerTaskRoutes(v1, scheduledTaskH)
	registerUserRoutes(v1, userH)
	registerGitWebhookRoutes(r, v1, gitWebhookH, webhookRateLimiter)
	registerReleaseTrackerRoutes(v1, releaseTrackerH)
	registerProxmoxRoutes(v1, proxmoxH)
	registerHostPermissionRoutes(v1, hostPermH)
	registerUptimeRoutes(v1, uptimeH)
	registerSSLRoutes(v1, sslH)
	registerNPMRoutes(v1, npmH)

	registerStaticFiles(r)

	cleanup := func() {
		ipRateLimiter.Stop()
		agentRateLimiter.Stop()
		webhookRateLimiter.Stop()
	}
	return r, releaseTrackerH, proxmoxH, npmH, cleanup
}

func registerPublicRoutes(r *gin.Engine, h *handlers.AuthHandler, db *database.DB) {
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/refresh", h.RefreshToken)
	r.POST("/api/auth/logout", h.Logout)
	r.GET("/api/health", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "degraded", "db": "unreachable", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok", "db": "ok"})
	})
}

func registerWSRoutes(r *gin.Engine, h *ws.WSHandler, cfg *config.Config) {
	g := r.Group("/api/v1/ws")
	g.Use(WSTokenMiddleware(cfg))
	g.GET("/dashboard", h.Dashboard)
	g.GET("/hosts/:id", h.HostDetail)
	g.GET("/docker", h.Docker)
	g.GET("/network", h.Network)
	g.GET("/apt", h.Apt)
	g.GET("/commands/stream/:command_id", h.CommandStream)
	g.GET("/notifications", h.NotificationStream)
}

func registerAgentRoutes(r *gin.Engine, db *database.DB, cfg *config.Config, h *handlers.AgentHandler, rl *IPRateLimiter) {
	g := r.Group("/api/agent")
	g.Use(RateLimiterMiddleware(rl))
	g.Use(APIKeyMiddleware(db, cfg))
	g.POST("/report", h.ReceiveReport)
	g.POST("/command/result", h.ReportCommandResult)
	g.POST("/command/stream", h.StreamCommandOutput)
	g.POST("/audit", h.LogAuditAction)
}

func registerAuthRoutes(g *gin.RouterGroup, h *handlers.AuthHandler) {
	g.GET("/auth/profile", h.GetProfile)
	g.POST("/auth/change-password", h.ChangePassword)
	g.GET("/auth/login-events", h.GetLoginEvents)
	g.GET("/auth/login-events/admin", h.GetAllLoginEventsAdmin)
	g.POST("/auth/revoke-all-sessions", h.RevokeAllSessions)
	g.GET("/auth/mfa/status", h.GetMFAStatus)
	g.POST("/auth/mfa/setup", h.SetupMFA)
	g.POST("/auth/mfa/verify", h.VerifyMFA)
	g.POST("/auth/mfa/disable", h.DisableMFA)
	g.GET("/auth/security", h.GetSecuritySummary)
	g.DELETE("/auth/blocked-ips/:ip", h.UnblockIP)
}

func registerWebLogsRoutes(g *gin.RouterGroup, h *handlers.WebLogsHandler) {
	g.GET("/security/web-logs", h.GetWebLogsSummary)
	g.GET("/security/web-logs/timeseries", h.GetWebLogsTimeseries)
	g.GET("/security/web-logs/live", h.GetWebLogsLive)
	g.GET("/security/web-logs/ip/:ip", h.GetWebLogsIPTimeline)
	g.POST("/security/web-logs/ip/:ip/decisions", h.BlockCrowdSecIP)
	g.DELETE("/security/web-logs/ip/:ip/decisions", h.UnblockCrowdSecIP)
	g.GET("/security/web-logs/domain/:domain", h.GetWebLogsDomainDetails)
}

func registerHostRoutes(g *gin.RouterGroup, h *handlers.HostHandler, agentH *handlers.AgentHandler, db *database.DB) {
	g.GET("/hosts", h.ListHosts)
	g.POST("/hosts", h.RegisterHost)
	g.GET("/metrics/summary", agentH.GetMetricsSummary)

	// Per-host routes protected by HostPermissionMiddleware (viewer level).
	hostViewer := g.Group("/hosts/:id")
	hostViewer.Use(HostPermissionMiddleware(db, "viewer"))
	hostViewer.GET("", h.GetHost)
	hostViewer.GET("/dashboard", h.GetHostDashboard)
	hostViewer.GET("/metrics/history", agentH.GetMetricsHistory)
	hostViewer.GET("/metrics/aggregated", agentH.GetMetricsAggregated)
	hostViewer.GET("/disk/metrics", h.GetDiskMetrics)
	hostViewer.GET("/disk/metrics/history", h.GetDiskMetricsHistory)
	hostViewer.GET("/disk/metrics/aggregated", h.GetDiskMetricsAggregated)
	hostViewer.GET("/disk/health", h.GetDiskHealth)
	hostViewer.GET("/complete", h.GetHostComplete)

	// Write operations on hosts require operator level.
	hostOperator := g.Group("/hosts/:id")
	hostOperator.Use(HostPermissionMiddleware(db, "operator"))
	hostOperator.PATCH("", h.UpdateHost)
	hostOperator.DELETE("", h.DeleteHost)
	hostOperator.POST("/rotate-key", h.RotateAPIKey)
	hostOperator.POST("/agent/update", h.TriggerAgentUpdate)
}

func registerHostPermissionRoutes(g *gin.RouterGroup, h *handlers.HostPermissionHandler) {
	// Admin-only: manage per-host permissions
	admin := g.Group("")
	admin.Use(AdminOnlyMiddleware())
	admin.GET("/hosts/:id/permissions", h.ListHostPermissions)
	admin.PUT("/hosts/:id/permissions/:username", h.SetHostPermission)
	admin.DELETE("/hosts/:id/permissions/:username", h.DeleteHostPermission)
	// Any authenticated user: view own permissions
	g.GET("/auth/host-permissions", h.GetMyHostPermissions)
}

func registerDockerRoutes(g *gin.RouterGroup, dockerH *handlers.DockerHandler, systemH *handlers.SystemHandler, networkH *handlers.NetworkHandler, agentH *handlers.AgentHandler) {
	g.GET("/hosts/:id/containers", dockerH.ListContainers)
	g.GET("/hosts/:id/commands/history", agentH.GetHostCommandHistory)
	g.GET("/hosts/:id/compose-projects", dockerH.ListHostComposeProjects)
	g.GET("/docker/containers", dockerH.ListAllContainers)
	g.GET("/docker/compose", dockerH.ListComposeProjects)
	g.POST("/docker/command", dockerH.SendDockerCommand)
	g.POST("/system/journalctl", systemH.SendJournalCommand)
	g.POST("/system/service", systemH.SendSystemdCommand)
	g.POST("/system/processes", systemH.SendProcessesCommand)
	g.GET("/network", networkH.GetNetworkSnapshot)
	g.GET("/network/topology", networkH.GetTopologySnapshot)
	g.GET("/network/config", networkH.GetTopologyConfig)
	g.PUT("/network/config", networkH.SaveTopologyConfig)
}

func registerAPTRoutes(g *gin.RouterGroup, h *handlers.AptHandler) {
	g.GET("/hosts/:id/apt", h.GetAptStatus)
	g.GET("/apt/summary", h.GetCVESummary)
	g.POST("/apt/command", h.SendCommand)

	// Unattended-upgrades
	g.GET("/hosts/:id/apt/unattended-upgrades", h.GetUUStatus)
	g.PUT("/hosts/:id/apt/unattended-upgrades", h.ConfigureUU)
	g.POST("/hosts/:id/apt/unattended-upgrades/install", h.InstallUU)
	g.POST("/hosts/:id/apt/unattended-upgrades/run-now", h.RunUUNow)
	g.GET("/hosts/:id/apt/unattended-upgrades/runs", h.GetUURuns)
}

func registerAuditRoutes(g *gin.RouterGroup, h *handlers.AuditHandler) {
	g.GET("/audit/logs", h.GetAuditLogs)
	g.GET("/audit/logs/me", h.GetMyAuditLogs)
	g.GET("/audit/logs/host/:host_id", h.GetAuditLogsByHost)
	g.GET("/audit/logs/user/:username", h.GetAuditLogsByUser)
	g.GET("/audit/commands", h.GetCommandsHistory)
	g.GET("/commands/:id", h.GetCommandByID)
}

func registerNotifRoutes(g *gin.RouterGroup, h *handlers.NotificationsHandler) {
	g.GET("/notifications", h.GetNotifications)
	g.POST("/notifications/mark-read", h.MarkRead)
}

func registerPushRoutes(g *gin.RouterGroup, h *handlers.PushHandler) {
	g.GET("/push/vapid-public-key", h.GetVapidPublicKey)
	g.POST("/push/subscribe", h.Subscribe)
	g.DELETE("/push/subscribe", h.Unsubscribe)
}

func registerAlertRoutes(g *gin.RouterGroup, rulesH *handlers.AlertRulesHandler) {
	admin := g.Group("")
	admin.Use(AdminOnlyMiddleware())
	admin.GET("/alerts/incidents", rulesH.ListIncidents)
	admin.POST("/alerts/incidents/:id/resolve", rulesH.ResolveIncident)
	admin.GET("/alert-rules/capabilities/agent", rulesH.GetAgentAlertRuleCapabilities)
	admin.GET("/alert-rules/capabilities/proxmox", rulesH.GetProxmoxAlertRuleCapabilities)
	admin.GET("/alert-rules/capabilities/synthetic", rulesH.GetSyntheticAlertRuleCapabilities)
	admin.GET("/alert-rules/capabilities/docker", rulesH.GetDockerAlertRuleCapabilities)
	admin.GET("/hosts/:id/capabilities", rulesH.GetHostAlertMetrics)
	admin.GET("/alert-rules", rulesH.ListAlertRules)
	admin.GET("/alert-rules/:id", rulesH.GetAlertRule)
	admin.POST("/alert-rules", rulesH.CreateAlertRule)
	admin.PATCH("/alert-rules/:id", rulesH.UpdateAlertRule)
	admin.DELETE("/alert-rules/:id", rulesH.DeleteAlertRule)
	admin.POST("/alert-rules/test", rulesH.TestAlertRule)
	admin.POST("/alert-rules/test/logs", rulesH.TestAlertRuleLogs)
}

func registerSettingsRoutes(g *gin.RouterGroup, h *handlers.SettingsHandler) {
	g.GET("/settings", h.GetSettings)
	g.PUT("/settings", h.UpdateSettings)
	g.POST("/settings/test-smtp", h.TestSmtp)
	g.POST("/settings/test-ntfy", h.TestNtfy)
	g.POST("/settings/cleanup-metrics", h.CleanupMetrics)
	g.POST("/settings/cleanup-audit", h.CleanupAuditLogs)
}

func registerTaskRoutes(g *gin.RouterGroup, h *handlers.ScheduledTaskHandler) {
	g.GET("/scheduled-tasks", h.ListAllScheduledTasks)
	g.GET("/hosts/:id/scheduled-tasks", h.ListScheduledTasks)
	g.POST("/hosts/:id/scheduled-tasks", h.CreateScheduledTask)
	g.GET("/hosts/:id/custom-tasks", h.GetCustomTasks)
	g.GET("/hosts/:id/tasks-yaml", h.GetTasksConfigYAML)
	g.PUT("/scheduled-tasks/:id", h.UpdateScheduledTask)
	g.DELETE("/scheduled-tasks/:id", h.DeleteScheduledTask)
	g.POST("/scheduled-tasks/:id/run", h.RunScheduledTask)
	g.GET("/scheduled-tasks/:id/executions", h.GetScheduledTaskExecutions)
}

func registerUserRoutes(g *gin.RouterGroup, h *handlers.UserHandler) {
	g.GET("/users", h.ListUsers)
	g.POST("/users", h.CreateUser)
	g.PATCH("/users/:id/role", h.UpdateUserRole)
	g.DELETE("/users/:id", h.DeleteUser)
}

func registerGitWebhookRoutes(r *gin.Engine, g *gin.RouterGroup, h *handlers.GitWebhookHandler, webhookRL *IPRateLimiter) {
	g.GET("/webhooks/git", h.ListWebhooks)
	g.POST("/webhooks/git", h.CreateWebhook)
	g.GET("/webhooks/git/:id", h.GetWebhook)
	g.PUT("/webhooks/git/:id", h.UpdateWebhook)
	g.DELETE("/webhooks/git/:id", h.DeleteWebhook)
	g.POST("/webhooks/git/:id/regenerate-secret", h.RegenerateSecret)
	g.GET("/webhooks/git/:id/executions", h.GetWebhookExecutions)
	// Public receiver — HMAC-authenticated, no JWT, dedicated stricter rate limit.
	recv := r.Group("/api/v1/webhooks/git")
	recv.Use(RateLimiterMiddleware(webhookRL))
	recv.POST("/:id/receive", h.ReceiveWebhook)
}

func registerReleaseTrackerRoutes(g *gin.RouterGroup, h *handlers.ReleaseTrackerHandler) {
	g.GET("/release-trackers", h.List)
	g.POST("/release-trackers", h.Create)
	g.POST("/release-trackers/bulk", h.CreateBulk)
	g.GET("/release-trackers/trackable-containers", h.ListTrackableContainers)
	g.GET("/release-trackers/:id", h.Get)
	g.PUT("/release-trackers/:id", h.Update)
	g.DELETE("/release-trackers/:id", h.Delete)
	g.POST("/release-trackers/:id/check-now", h.TriggerCheck)
	g.POST("/release-trackers/:id/run", h.Run)
	g.GET("/release-trackers/:id/executions", h.GetExecutions)
	g.GET("/release-trackers/:id/version-history", h.GetVersionHistory)

	g.GET("/registry-credentials", h.ListRegistryCredentials)
	g.POST("/registry-credentials", h.CreateRegistryCredential)
	g.PUT("/registry-credentials/:id", h.UpdateRegistryCredential)
	g.DELETE("/registry-credentials/:id", h.DeleteRegistryCredential)
}

func registerProxmoxRoutes(g *gin.RouterGroup, h *handlers.ProxmoxHandler) {
	// Summary & read-only data (all authenticated users)
	g.GET("/proxmox/summary", h.GetSummary)
	g.GET("/proxmox/nodes", h.ListNodes)
	g.GET("/proxmox/nodes/metrics", h.GetNodeMetricsSummary)
	g.GET("/proxmox/nodes/:id", h.GetNode)
	g.GET("/proxmox/nodes/:id/cpu-temp/history", h.GetNodeCPUTemperatureHistory)
	g.GET("/proxmox/nodes/:id/fan-rpm/history", h.GetNodeFanRPMHistory)
	g.GET("/proxmox/nodes/:id/sensor-source/candidates", h.ListNodeSensorSourceCandidates)
	g.GET("/proxmox/guests", h.ListGuests)
	g.GET("/proxmox/guests/:id/metrics", h.GetGuestMetricsSummary)
	g.GET("/proxmox/guests/:id/link", h.GetLinkByGuest)
	// Connection management — admin only
	proxmoxAdmin := g.Group("")
	proxmoxAdmin.Use(AdminOnlyMiddleware())
	proxmoxAdmin.GET("/proxmox/instances", h.ListConnections)
	proxmoxAdmin.POST("/proxmox/instances", h.CreateConnection)
	proxmoxAdmin.GET("/proxmox/instances/:id", h.GetConnection)
	proxmoxAdmin.PUT("/proxmox/instances/:id", h.UpdateConnection)
	proxmoxAdmin.DELETE("/proxmox/instances/:id", h.DeleteConnection)
	proxmoxAdmin.POST("/proxmox/instances/test", h.TestConnection)
	proxmoxAdmin.POST("/proxmox/instances/:id/test", h.TestConnectionByID)
	proxmoxAdmin.POST("/proxmox/instances/:id/poll-now", h.PollNow)
	proxmoxAdmin.PUT("/proxmox/nodes/:id/sensor-source", h.UpdateNodeSensorSource)
	// Guest ↔ host link management
	g.GET("/proxmox/links", h.ListLinks)
	g.POST("/proxmox/links", h.CreateLink)
	g.GET("/proxmox/links/:id", h.GetLink)
	g.PUT("/proxmox/links/:id", h.UpdateLink)
	g.DELETE("/proxmox/links/:id", h.DeleteLink)
	// Per-host Proxmox link lookup + candidate guests for manual linking
	g.GET("/hosts/:id/proxmox-link", h.GetLinkByHost)
	g.GET("/hosts/:id/proxmox-candidates", h.ListLinkCandidates)
	g.GET("/hosts/:id/proxmox-disks", h.GetHostProxmoxDisks)

	// Extended read-only data (tasks, backups, disks)
	g.GET("/proxmox/tasks", h.ListTasks)
	g.GET("/proxmox/nodes/:id/tasks", h.ListNodeTasks)
	g.GET("/proxmox/nodes/:id/disks", h.ListNodeDisks)
	g.GET("/proxmox/backup-jobs", h.ListBackupJobs)
	g.GET("/proxmox/backup-runs", h.ListBackupRuns)

	// Node live data (proxied from PVE, not cached in DB)
	g.GET("/proxmox/nodes/:id/status", h.GetNodeStatus)
	g.GET("/proxmox/nodes/:id/syslog", h.GetNodeSyslog)
	g.GET("/proxmox/nodes/:id/tasks/:upid/log", h.GetTaskLog)
	g.GET("/proxmox/nodes/:id/rrd", h.GetNodeRRD)

	// Node services (list requires Sys.Audit; actions require Sys.Modify)
	g.GET("/proxmox/nodes/:id/services", h.ListNodeServices)
	g.POST("/proxmox/nodes/:id/services/:service/:action", h.NodeServiceAction)

	// Guest network interfaces (live — VM via QEMU agent, LXC native)
	g.GET("/proxmox/nodes/:id/guest-networks", h.GetNodeGuestNetworks)

	// Node actions (write — require Sys.Modify on the Proxmox token)
	g.POST("/proxmox/nodes/:id/apt-refresh", h.RefreshNodeApt)
	g.POST("/proxmox/nodes/:id/guests/:vmid/migrate", h.MigrateGuest)
}

func registerUptimeRoutes(g *gin.RouterGroup, h *handlers.UptimeHandler) {
	// Read endpoints: any authenticated user
	g.GET("/uptime/probes", h.List)
	g.GET("/uptime/probes/:id", h.Get)
	g.GET("/uptime/probes/:id/history", h.History)
	g.GET("/uptime/probes/:id/stats", h.Stats)

	// Write endpoints: admin only
	admin := g.Group("")
	admin.Use(AdminOnlyMiddleware())
	admin.POST("/uptime/probes", h.Create)
	admin.PUT("/uptime/probes/:id", h.Update)
	admin.DELETE("/uptime/probes/:id", h.Delete)
	admin.POST("/uptime/probes/:id/check-now", h.CheckNow)
}

func registerSSLRoutes(g *gin.RouterGroup, h *handlers.SSLHandler) {
	g.GET("/ssl/certificates", h.List)
	g.GET("/ssl/certificates/:id", h.Get)
	g.GET("/ssl/certificates/:id/history", h.History)

	admin := g.Group("")
	admin.Use(AdminOnlyMiddleware())
	admin.POST("/ssl/certificates", h.Create)
	admin.PUT("/ssl/certificates/:id", h.Update)
	admin.DELETE("/ssl/certificates/:id", h.Delete)
	admin.POST("/ssl/certificates/:id/check-now", h.CheckNow)
}

func registerNPMRoutes(g *gin.RouterGroup, h *handlers.NPMHandler) {
	// Read endpoints: any authenticated user
	g.GET("/npm/connections", h.ListConnections)
	g.GET("/npm/connections/:id/proxy-hosts", h.ListProxyHosts)
	g.GET("/npm/proxy-hosts", h.ListAllProxyHosts)

	// Write endpoints: admin only
	admin := g.Group("")
	admin.Use(AdminOnlyMiddleware())
	admin.POST("/npm/connections", h.CreateConnection)
	admin.PUT("/npm/connections/:id", h.UpdateConnection)
	admin.DELETE("/npm/connections/:id", h.DeleteConnection)
	admin.POST("/npm/connections/test", h.TestConnection)
	admin.POST("/npm/connections/:id/refresh-now", h.RefreshNow)
	admin.PATCH("/npm/proxy-hosts/:id", h.UpdateProxyHost)
	admin.PATCH("/npm/proxy-hosts/:id/npm-enabled", h.SetNPMEnabled)
}

func registerStaticFiles(r *gin.Engine) {
	r.Static("/assets", "./frontend/dist/assets")
	r.StaticFile("/", "./frontend/dist/index.html")
	// Static root-level files must be explicit — the NoRoute SPA fallback would
	// otherwise serve index.html for them, causing parse errors in the browser.
	r.StaticFile("/manifest.json", "./frontend/dist/manifest.json")
	r.StaticFile("/favicon.svg", "./frontend/dist/favicon.svg")
	r.StaticFile("/service-worker.js", "./frontend/dist/service-worker.js")
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})
}

package api

import (
	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/scheduler"
	"github.com/serversupervisor/server/internal/ws"
)

// SetupRouter wires all handlers and registers route groups.
// The caller is responsible for starting long-running poller services after this function returns.
func SetupRouter(db *database.DB, cfg *config.Config, notifHub *ws.NotificationHub, sched *scheduler.TaskScheduler, dispatcher *dispatch.Dispatcher) (*gin.Engine, *handlers.ReleaseTrackerHandler, *handlers.ProxmoxHandler) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestLogger())
	r.Use(SecurityHeadersMiddleware())
	r.Use(CORSMiddleware(cfg.BaseURL, cfg.AllowedOrigins))

	ipRateLimiter := NewIPRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst, cfg.TrustedProxyCIDRs)
	r.Use(RateLimiterMiddleware(ipRateLimiter))

	// Instantiate handlers
	authH := handlers.NewAuthHandler(db, cfg)
	hostH := handlers.NewHostHandler(db, cfg)
	wsH := ws.NewWSHandler(db, cfg, notifHub)
	agentH := handlers.NewAgentHandler(db, cfg, wsH.GetStreamHub())
	aptH := handlers.NewAptHandler(db, cfg, dispatcher)
	dockerH := handlers.NewDockerHandler(db, cfg, dispatcher, wsH.GetStreamHub())
	systemH := handlers.NewSystemHandler(db, cfg, dispatcher, wsH.GetStreamHub())
	networkH := handlers.NewNetworkHandler(db)
	auditH := handlers.NewAuditHandler(db, cfg)
	userH := handlers.NewUserHandler(db, cfg)
	alertH := handlers.NewAlertHandler(db, cfg)
	alertRulesH := handlers.NewAlertRulesHandler(db, cfg)
	settingsH := handlers.NewSettingsHandler(db, cfg)
	notifH := handlers.NewNotificationsHandler(db)
	pushH := handlers.NewPushHandler(db, cfg)
	scheduledTaskH := handlers.NewScheduledTaskHandler(db, cfg, dispatcher, sched)
	gitWebhookH := handlers.NewGitWebhookHandler(db, cfg, dispatcher, notifHub)
	releaseTrackerH := handlers.NewReleaseTrackerHandler(db, cfg, dispatcher, notifHub)
	agentH.AddCompletionListener(gitWebhookH)
	agentH.AddCompletionListener(releaseTrackerH)

	proxmoxH := handlers.NewProxmoxHandler(db, cfg)

	registerPublicRoutes(r, authH)
	registerWSRoutes(r, wsH)
	registerAgentRoutes(r, db, cfg, agentH)

	v1 := r.Group("/api/v1")
	v1.Use(JWTMiddleware(cfg))
	registerAuthRoutes(v1, authH)
	registerHostRoutes(v1, hostH, agentH)
	registerDockerRoutes(v1, dockerH, systemH, networkH, agentH)
	registerAPTRoutes(v1, aptH)
	registerAuditRoutes(v1, auditH)
	registerAlertRoutes(v1, alertH, alertRulesH)
	registerNotifRoutes(v1, notifH)
	registerPushRoutes(v1, pushH)
	registerSettingsRoutes(v1, settingsH)
	registerTaskRoutes(v1, scheduledTaskH)
	registerUserRoutes(v1, userH)
	registerGitWebhookRoutes(r, v1, gitWebhookH)
	registerReleaseTrackerRoutes(v1, releaseTrackerH)
	registerProxmoxRoutes(v1, proxmoxH)

	registerStaticFiles(r)

	return r, releaseTrackerH, proxmoxH
}

func registerPublicRoutes(r *gin.Engine, h *handlers.AuthHandler) {
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/refresh", h.RefreshToken)
	r.POST("/api/auth/logout", h.Logout)
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

func registerWSRoutes(r *gin.Engine, h *ws.WSHandler) {
	r.GET("/api/v1/ws/dashboard", h.Dashboard)
	r.GET("/api/v1/ws/hosts/:id", h.HostDetail)
	r.GET("/api/v1/ws/docker", h.Docker)
	r.GET("/api/v1/ws/network", h.Network)
	r.GET("/api/v1/ws/apt", h.Apt)
	r.GET("/api/v1/ws/commands/stream/:command_id", h.CommandStream)
	r.GET("/api/v1/ws/notifications", h.NotificationStream)
}

func registerAgentRoutes(r *gin.Engine, db *database.DB, cfg *config.Config, h *handlers.AgentHandler) {
	g := r.Group("/api/agent")
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

func registerHostRoutes(g *gin.RouterGroup, h *handlers.HostHandler, agentH *handlers.AgentHandler) {
	g.GET("/hosts", h.ListHosts)
	g.POST("/hosts", h.RegisterHost)
	g.GET("/hosts/:id", h.GetHost)
	g.PATCH("/hosts/:id", h.UpdateHost)
	g.DELETE("/hosts/:id", h.DeleteHost)
	g.POST("/hosts/:id/rotate-key", h.RotateAPIKey)
	g.GET("/hosts/:id/dashboard", h.GetHostDashboard)
	g.GET("/hosts/:id/metrics/history", agentH.GetMetricsHistory)
	g.GET("/hosts/:id/metrics/aggregated", agentH.GetMetricsAggregated)
	g.GET("/metrics/summary", agentH.GetMetricsSummary)
	g.GET("/hosts/:id/disk/metrics", h.GetDiskMetrics)
	g.GET("/hosts/:id/disk/metrics/history", h.GetDiskMetricsHistory)
	g.GET("/hosts/:id/disk/health", h.GetDiskHealth)
	g.GET("/hosts/:id/complete", h.GetHostComplete)
}

func registerDockerRoutes(g *gin.RouterGroup, dockerH *handlers.DockerHandler, systemH *handlers.SystemHandler, networkH *handlers.NetworkHandler, agentH *handlers.AgentHandler) {
	g.GET("/hosts/:id/containers", dockerH.ListContainers)
	g.GET("/hosts/:id/commands/history", agentH.GetHostCommandHistory)
	g.GET("/docker/containers", dockerH.ListAllContainers)
	g.GET("/docker/compose", dockerH.ListComposeProjects)
	g.GET("/docker/versions", dockerH.CompareVersions)
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

func registerAlertRoutes(g *gin.RouterGroup, alertH *handlers.AlertHandler, rulesH *handlers.AlertRulesHandler) {
	g.GET("/alerts/incidents", alertH.ListIncidents)
	g.GET("/alert-rules", rulesH.ListAlertRules)
	g.GET("/alert-rules/:id", rulesH.GetAlertRule)
	g.POST("/alert-rules", rulesH.CreateAlertRule)
	g.PATCH("/alert-rules/:id", rulesH.UpdateAlertRule)
	g.DELETE("/alert-rules/:id", rulesH.DeleteAlertRule)
	g.POST("/alert-rules/test", rulesH.TestAlertRule)
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

func registerGitWebhookRoutes(r *gin.Engine, g *gin.RouterGroup, h *handlers.GitWebhookHandler) {
	g.GET("/webhooks/git", h.ListWebhooks)
	g.POST("/webhooks/git", h.CreateWebhook)
	g.GET("/webhooks/git/:id", h.GetWebhook)
	g.PUT("/webhooks/git/:id", h.UpdateWebhook)
	g.DELETE("/webhooks/git/:id", h.DeleteWebhook)
	g.POST("/webhooks/git/:id/regenerate-secret", h.RegenerateSecret)
	g.GET("/webhooks/git/:id/executions", h.GetWebhookExecutions)
	// Public receiver — HMAC-authenticated, no JWT
	r.POST("/api/v1/webhooks/git/:id/receive", h.ReceiveWebhook)
}

func registerReleaseTrackerRoutes(g *gin.RouterGroup, h *handlers.ReleaseTrackerHandler) {
	g.GET("/release-trackers", h.List)
	g.POST("/release-trackers", h.Create)
	g.GET("/release-trackers/:id", h.Get)
	g.PUT("/release-trackers/:id", h.Update)
	g.DELETE("/release-trackers/:id", h.Delete)
	g.POST("/release-trackers/:id/check-now", h.TriggerCheck)
	g.POST("/release-trackers/:id/run", h.Run)
	g.GET("/release-trackers/:id/executions", h.GetExecutions)
}

func registerProxmoxRoutes(g *gin.RouterGroup, h *handlers.ProxmoxHandler) {
	// Summary & read-only data (all authenticated users)
	g.GET("/proxmox/summary", h.GetSummary)
	g.GET("/proxmox/nodes", h.ListNodes)
	g.GET("/proxmox/nodes/:id", h.GetNode)
	g.GET("/proxmox/guests", h.ListGuests)
	g.GET("/proxmox/guests/:id/link", h.GetLinkByGuest)
	// Connection management (admin only enforced in handler via RequireAdmin middleware if needed;
	// for now protected by JWT — tighten with AdminMiddleware if desired)
	g.GET("/proxmox/instances", h.ListConnections)
	g.POST("/proxmox/instances", h.CreateConnection)
	g.GET("/proxmox/instances/:id", h.GetConnection)
	g.PUT("/proxmox/instances/:id", h.UpdateConnection)
	g.DELETE("/proxmox/instances/:id", h.DeleteConnection)
	g.POST("/proxmox/instances/test", h.TestConnection)
	g.POST("/proxmox/instances/:id/test", h.TestConnectionByID)
	g.POST("/proxmox/instances/:id/poll-now", h.PollNow)
	// Guest ↔ host link management
	g.GET("/proxmox/links", h.ListLinks)
	g.POST("/proxmox/links", h.CreateLink)
	g.GET("/proxmox/links/:id", h.GetLink)
	g.PUT("/proxmox/links/:id", h.UpdateLink)
	g.DELETE("/proxmox/links/:id", h.DeleteLink)
	// Per-host Proxmox link lookup + candidate guests for manual linking
	g.GET("/hosts/:id/proxmox-link", h.GetLinkByHost)
	g.GET("/hosts/:id/proxmox-candidates", h.ListLinkCandidates)

	// Extended read-only data (tasks, backups, disks)
	g.GET("/proxmox/tasks", h.ListTasks)
	g.GET("/proxmox/nodes/:id/tasks", h.ListNodeTasks)
	g.GET("/proxmox/nodes/:id/disks", h.ListNodeDisks)
	g.GET("/proxmox/backup-jobs", h.ListBackupJobs)
	g.GET("/proxmox/backup-runs", h.ListBackupRuns)

	// Node live data (proxied from PVE, not cached in DB)
	g.GET("/proxmox/nodes/:id/status", h.GetNodeStatus)
	g.GET("/proxmox/nodes/:id/tasks/:upid/log", h.GetTaskLog)
	g.GET("/proxmox/nodes/:id/rrd", h.GetNodeRRD)

	// Node services (list requires Sys.Audit; actions require Sys.Modify)
	g.GET("/proxmox/nodes/:id/services", h.ListNodeServices)
	g.POST("/proxmox/nodes/:id/services/:service/:action", h.NodeServiceAction)

	// Guest network interfaces (live — VM via QEMU agent, LXC native)
	g.GET("/proxmox/nodes/:id/guest-networks", h.GetNodeGuestNetworks)

	// Node actions (write — require Sys.Modify on the Proxmox token)
	g.POST("/proxmox/nodes/:id/apt-refresh", h.RefreshNodeApt)
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

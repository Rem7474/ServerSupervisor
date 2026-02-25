package api

import (
	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

func SetupRouter(db *database.DB, cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestLogger())
	r.Use(CORSMiddleware(cfg.BaseURL))

	// Per-IP rate limiter
	ipRateLimiter := NewIPRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst, cfg.TrustedProxyCIDRs)
	r.Use(RateLimiterMiddleware(ipRateLimiter))

	// Handlers
	authH := NewAuthHandler(db, cfg)
	hostH := NewHostHandler(db, cfg)
	wsH := NewWSHandler(db, cfg)
	agentH := NewAgentHandler(db, cfg, wsH.GetStreamHub())
	aptH := NewAptHandler(db, cfg)
	dockerH := NewDockerHandler(db, cfg, wsH.GetStreamHub())
	networkH := NewNetworkHandler(db)
	auditH := NewAuditHandler(db, cfg)
	userH := NewUserHandler(db, cfg)
	alertH := NewAlertHandler(db, cfg)
	alertRulesH := NewAlertRulesHandler(db, cfg)
	settingsH := NewSettingsHandler(db, cfg)

	// ========== Public routes ==========
	r.POST("/api/auth/login", authH.Login)
	r.POST("/api/auth/refresh", authH.RefreshToken)
	r.POST("/api/auth/logout", authH.Logout)

	// Health check
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// ========== WebSocket routes (token via query string) ==========
	r.GET("/api/v1/ws/dashboard", wsH.Dashboard)
	r.GET("/api/v1/ws/hosts/:id", wsH.HostDetail)
	r.GET("/api/v1/ws/docker", wsH.Docker)
	r.GET("/api/v1/ws/network", wsH.Network)
	r.GET("/api/v1/ws/apt", wsH.Apt)
	r.GET("/api/v1/ws/apt/stream/:command_id", wsH.AptStream)

	// ========== Agent routes (API Key auth) ==========
	agent := r.Group("/api/agent")
	agent.Use(APIKeyMiddleware(db, cfg))
	{
		agent.POST("/report", agentH.ReceiveReport)
		agent.POST("/command/result", agentH.ReportCommandResult)
		agent.POST("/command/stream", agentH.StreamCommandOutput)
		agent.POST("/audit", agentH.LogAuditAction)
	}

	// ========== Dashboard routes (JWT auth) ==========
	api := r.Group("/api/v1")
	api.Use(JWTMiddleware(cfg))
	{
		// Auth
		api.GET("/auth/profile", authH.GetProfile)
		api.POST("/auth/change-password", authH.ChangePassword)
		api.GET("/auth/login-events", authH.GetLoginEvents)
		api.GET("/auth/mfa/status", authH.GetMFAStatus)
		api.POST("/auth/mfa/setup", authH.SetupMFA)
		api.POST("/auth/mfa/verify", authH.VerifyMFA)
		api.POST("/auth/mfa/disable", authH.DisableMFA)

		// Hosts
		api.GET("/hosts", hostH.ListHosts)
		api.POST("/hosts", hostH.RegisterHost)
		api.GET("/hosts/:id", hostH.GetHost)
		api.PATCH("/hosts/:id", hostH.UpdateHost)
		api.DELETE("/hosts/:id", hostH.DeleteHost)
		api.POST("/hosts/:id/rotate-key", hostH.RotateAPIKey)
		api.GET("/hosts/:id/dashboard", hostH.GetHostDashboard)

		// Metrics
		api.GET("/hosts/:id/metrics/history", agentH.GetMetricsHistory)
		api.GET("/hosts/:id/metrics/aggregated", agentH.GetMetricsAggregated)
		api.GET("/metrics/summary", agentH.GetMetricsSummary)

		// Disk metrics and health
		api.GET("/hosts/:id/disk/metrics", hostH.GetDiskMetrics)
		api.GET("/hosts/:id/disk/metrics/history", hostH.GetDiskMetricsHistory)
		api.GET("/hosts/:id/disk/health", hostH.GetDiskHealth)

		// Docker
		api.GET("/hosts/:id/containers", dockerH.ListContainers)
		api.GET("/hosts/:id/docker/history", dockerH.GetDockerCommandHistory)
		api.GET("/docker/containers", dockerH.ListAllContainers)
		api.GET("/docker/versions", dockerH.CompareVersions)
		api.POST("/docker/command", dockerH.SendDockerCommand)
		api.POST("/system/journalctl", dockerH.SendJournalCommand)
		api.GET("/network", networkH.GetNetworkSnapshot)
		api.GET("/network/topology", networkH.GetTopologySnapshot)
		api.GET("/network/config", networkH.GetTopologyConfig)
		api.PUT("/network/config", networkH.SaveTopologyConfig)

		// Tracked GitHub repos
		api.GET("/repos", dockerH.ListTrackedRepos)
		api.POST("/repos", dockerH.AddTrackedRepo)
		api.DELETE("/repos/:id", dockerH.DeleteTrackedRepo)

		// APT
		api.GET("/hosts/:id/apt", aptH.GetAptStatus)
		api.GET("/hosts/:id/apt/history", aptH.GetCommandHistory)
		api.POST("/apt/command", aptH.SendCommand)

		// Audit logs
		api.GET("/audit/logs", auditH.GetAuditLogs)
		api.GET("/audit/logs/me", auditH.GetMyAuditLogs)
		api.GET("/audit/logs/host/:host_id", auditH.GetAuditLogsByHost)
		api.GET("/audit/logs/user/:username", auditH.GetAuditLogsByUser)

		// Alerts
		api.GET("/alerts/rules", alertH.ListRules)
		api.POST("/alerts/rules", alertH.CreateRule)
		api.PATCH("/alerts/rules/:id", alertH.UpdateRule)
		api.DELETE("/alerts/rules/:id", alertH.DeleteRule)
		api.GET("/alerts/incidents", alertH.ListIncidents)

		// Configurable Alert Rules (new system)
		api.GET("/alert-rules", alertRulesH.ListAlertRules)
		api.GET("/alert-rules/:id", alertRulesH.GetAlertRule)
		api.POST("/alert-rules", alertRulesH.CreateAlertRule)
		api.PATCH("/alert-rules/:id", alertRulesH.UpdateAlertRule)
		api.DELETE("/alert-rules/:id", alertRulesH.DeleteAlertRule)
		api.POST("/alert-rules/test", alertRulesH.TestAlertRule)

		// Settings
		api.GET("/settings", settingsH.GetSettings)
		api.POST("/settings/test-smtp", settingsH.TestSmtp)
		api.POST("/settings/test-ntfy", settingsH.TestNtfy)
		api.POST("/settings/cleanup-metrics", settingsH.CleanupMetrics)
		api.POST("/settings/cleanup-audit", settingsH.CleanupAuditLogs)

		// Users
		api.GET("/users", userH.ListUsers)
		api.POST("/users", userH.CreateUser)
		api.PATCH("/users/:id/role", userH.UpdateUserRole)
		api.DELETE("/users/:id", userH.DeleteUser)
	}

	// Serve frontend static files
	r.Static("/assets", "./frontend/dist/assets")
	r.StaticFile("/", "./frontend/dist/index.html")
	// Static root-level files must be explicit — the NoRoute SPA fallback would
	// otherwise serve index.html for them, causing parse errors in the browser.
	r.StaticFile("/manifest.json", "./frontend/dist/manifest.json")
	r.StaticFile("/favicon.svg", "./frontend/dist/favicon.svg")
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	return r
}

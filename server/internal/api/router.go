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
	r.Use(CORSMiddleware("http://localhost:8080"))

	// Per-IP rate limiter
	ipRateLimiter := NewIPRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	r.Use(RateLimiterMiddleware(ipRateLimiter))

	// Handlers
	authH := NewAuthHandler(db, cfg)
	hostH := NewHostHandler(db, cfg)
	agentH := NewAgentHandler(db, cfg)
	aptH := NewAptHandler(db, cfg)
	dockerH := NewDockerHandler(db, cfg)
	auditH := NewAuditHandler(db, cfg)

	// ========== Public routes ==========
	r.POST("/api/auth/login", authH.Login)

	// Health check
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// ========== Agent routes (API Key auth) ==========
	agent := r.Group("/api/agent")
	agent.Use(APIKeyMiddleware(db, cfg))
	{
		agent.POST("/report", agentH.ReceiveReport)
		agent.POST("/command/result", agentH.ReportCommandResult)
	}

	// ========== Dashboard routes (JWT auth) ==========
	api := r.Group("/api/v1")
	api.Use(JWTMiddleware(cfg))
	{
		// Auth
		api.POST("/auth/change-password", authH.ChangePassword)
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
		api.GET("/hosts/:id/dashboard", hostH.GetHostDashboard)

		// Metrics
		api.GET("/hosts/:id/metrics/history", agentH.GetMetricsHistory)

		// Docker
		api.GET("/hosts/:id/containers", dockerH.ListContainers)
		api.GET("/docker/containers", dockerH.ListAllContainers)
		api.GET("/docker/versions", dockerH.CompareVersions)

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
		api.GET("/audit/logs/host/:host_id", auditH.GetAuditLogsByHost)
		api.GET("/audit/logs/user/:username", auditH.GetAuditLogsByUser)
	}

	// Serve frontend static files
	r.Static("/assets", "./frontend/dist/assets")
	r.StaticFile("/", "./frontend/dist/index.html")
	r.NoRoute(func(c *gin.Context) {
		c.File("./frontend/dist/index.html")
	})

	return r
}

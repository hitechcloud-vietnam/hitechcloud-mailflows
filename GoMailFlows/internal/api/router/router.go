package router

import (
	"github.com/gin-gonic/gin"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/api/handlers"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/api/middleware"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/config"
	"gorm.io/gorm"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func Setup(cfg *config.Config, db *gorm.DB) *gin.Engine {
	if cfg.Server.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(middleware.CORS([]string{"*"}))
	r.Use(middleware.SecurityHeaders())

	// ============================================================
	// Swagger / OpenAPI
	// ============================================================
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.File("./docs/openapi.yaml")
	})
	r.GET("/openapi.json", func(c *gin.Context) {
		c.File("./docs/openapi.json")
	})

	// ============================================================
	// Health check
	// ============================================================
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "HiTechCloud MailFlows",
			"version": "1.0.0",
		})
	})

	r.GET("/ready", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			c.JSON(503, gin.H{"status": "not ready"})
			return
		}
		c.JSON(200, gin.H{"status": "ready"})
	})

	// ============================================================
	// Initialize handlers
	// ============================================================
	authHandler := handlers.NewAuthHandler(db, cfg)
	domainHandler := handlers.NewDomainHandler(db)
	smtpHandler := handlers.NewSMTPConfigHandler(db)
	pkgHandler := handlers.NewPackageHandler(db)
	emailHandler := handlers.NewEmailAccountHandler(db)
	marketingHandler := handlers.NewMarketingHandler(db)
	adminHandler := handlers.NewAdminHandler(db)
	mailHandler := handlers.NewMailHandler(db)

	// ============================================================
	// API v1
	// ============================================================
	v1 := r.Group("/api/v1")

	// --- Public routes ---
	public := v1.Group("")
	{
		// Auth
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
		public.POST("/auth/refresh", authHandler.RefreshToken)

		// Packages (public listing)
		public.GET("/packages", pkgHandler.List)
		public.GET("/packages/feature-groups", pkgHandler.GetFeatureGroups)
		public.GET("/packages/:id", pkgHandler.Get)

		// Marketing unsubscribe (public link)
		public.GET("/marketing/unsubscribe/:listId/:email", marketingHandler.Unsubscribe)
	}

	// --- Authenticated routes ---
	auth := v1.Group("")
	auth.Use(middleware.AuthMiddleware(&cfg.JWT))
	{
		// Auth
		auth.GET("/auth/me", authHandler.Me)
		auth.POST("/auth/logout", authHandler.Logout)
		auth.POST("/auth/change-password", authHandler.ChangePassword)

		// Domains
		auth.GET("/domains", domainHandler.List)
		auth.POST("/domains", domainHandler.Create)
		auth.GET("/domains/:id", domainHandler.Get)
		auth.DELETE("/domains/:id", domainHandler.Delete)
		auth.POST("/domains/:id/verify", domainHandler.Verify)
		auth.POST("/domains/:id/dns/regenerate", domainHandler.RegenerateDNS)

		// Email Accounts
		auth.GET("/email-accounts", emailHandler.List)
		auth.POST("/email-accounts", emailHandler.Create)
		auth.PUT("/email-accounts/:id", emailHandler.Update)
		auth.DELETE("/email-accounts/:id", emailHandler.Delete)

		// Mail operations
		auth.GET("/mail/folders", mailHandler.ListFolders)
		auth.GET("/mail/messages", mailHandler.ListMessages)
		auth.GET("/mail/messages/:id", mailHandler.GetMessage)
		auth.POST("/mail/send", mailHandler.SendMessage)
		auth.PUT("/mail/messages/:id/read", mailHandler.MarkRead)
		auth.PUT("/mail/messages/:id/unread", mailHandler.MarkUnread)
		auth.DELETE("/mail/messages/:id", mailHandler.DeleteMessage)
		auth.POST("/mail/messages/:id/move", mailHandler.MoveMessage)

		// Marketing
		auth.GET("/marketing/campaigns", marketingHandler.ListCampaigns)
		auth.POST("/marketing/campaigns", marketingHandler.CreateCampaign)
		auth.GET("/marketing/campaigns/:id", marketingHandler.GetCampaign)
		auth.POST("/marketing/campaigns/:id/schedule", marketingHandler.ScheduleCampaign)
		auth.POST("/marketing/campaigns/:id/send", marketingHandler.SendCampaign)
		auth.GET("/marketing/campaigns/:id/stats", marketingHandler.CampaignStats)
		auth.GET("/marketing/lists", marketingHandler.Lists)
		auth.POST("/marketing/lists", marketingHandler.CreateList)
		auth.POST("/marketing/lists/:listId/subscribers", marketingHandler.AddSubscriber)
		auth.POST("/marketing/subscribers/bulk", marketingHandler.BulkAddSubscribers)

		// Packages (authenticated)
		auth.PUT("/packages/:id", pkgHandler.Update)
	}

	// --- Admin routes ---
	admin := v1.Group("/admin")
	admin.Use(middleware.AuthMiddleware(&cfg.JWT))
	admin.Use(middleware.AdminMiddleware())
	{
		// Dashboard
		admin.GET("/dashboard", adminHandler.Dashboard)

		// Users
		admin.GET("/users", adminHandler.ListUsers)
		admin.PUT("/users/:id", adminHandler.UpdateUser)
		admin.POST("/users/:id/suspend", adminHandler.SuspendUser)
		admin.POST("/users/:id/activate", adminHandler.ActivateUser)

		// SMTP Configs
		admin.GET("/smtp-configs", smtpHandler.List)
		admin.POST("/smtp-configs", smtpHandler.Create)
		admin.PUT("/smtp-configs/:id", smtpHandler.Update)
		admin.DELETE("/smtp-configs/:id", smtpHandler.Delete)
		admin.POST("/smtp-configs/:id/assign", smtpHandler.AssignToDomain)
		admin.POST("/smtp-configs/:id/test", smtpHandler.TestConnection)

		// Packages
		admin.POST("/packages", pkgHandler.Create)

		// DNS Templates
		admin.GET("/dns-templates", adminHandler.ListDNSTemplates)
		admin.POST("/dns-templates", adminHandler.CreateDNSTemplate)
		admin.PUT("/dns-templates/:id", adminHandler.UpdateDNSTemplate)
		admin.DELETE("/dns-templates/:id", adminHandler.DeleteDNSTemplate)

		// System Settings
		admin.GET("/settings", adminHandler.GetSettings)
		admin.PUT("/settings/:key", adminHandler.UpdateSetting)

		// Audit Logs
		admin.GET("/audit-logs", adminHandler.GetAuditLogs)
	}

	return r
}

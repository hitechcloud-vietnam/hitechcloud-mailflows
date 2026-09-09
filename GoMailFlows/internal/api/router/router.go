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

	// Compatibility alias for /api routes expected by frontend
	apiLegacy := r.Group("/api")

	// --- Public routes ---
	setupPublicRoutes := func(g *gin.RouterGroup) {
		g.POST("/auth/register", authHandler.Register)
		g.POST("/auth/login", authHandler.Login)
		g.POST("/auth/refresh", authHandler.RefreshToken)
		g.GET("/auth/registration-status", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"open":                  false,
				"internalAuthDisabled": false,
			})
		})
		g.GET("/auth/oidc/providers", func(c *gin.Context) {
			c.JSON(200, gin.H{"providers": []string{}})
		})

		// Packages (public listing)
		g.GET("/packages", pkgHandler.List)
		g.GET("/packages/feature-groups", pkgHandler.GetFeatureGroups)
		g.GET("/packages/:id", pkgHandler.Get)

		// Marketing unsubscribe (public link)
		g.GET("/marketing/unsubscribe/:listId/:email", marketingHandler.Unsubscribe)
	}

	setupPublicRoutes(v1)
	setupPublicRoutes(apiLegacy)

	// --- Authenticated routes ---
	setupAuthRoutes := func(g *gin.RouterGroup) {
		g.Use(middleware.AuthMiddleware(&cfg.JWT))

		// Auth
		g.GET("/auth/me", authHandler.Me)
		g.POST("/auth/logout", authHandler.Logout)
		g.POST("/auth/change-password", authHandler.ChangePassword)
		g.GET("/auth/preferences", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"theme":          "default",
				"font":           "default",
				"fontSize":       100,
				"layout":         "comfortable",
				"pageSize":       50,
				"enabledPlugins": []string{},
			})
		})
		g.PATCH("/auth/preferences", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// Accounts
		g.GET("/accounts", func(c *gin.Context) {
			c.JSON(200, []interface{}{})
		})

		// Mail unread counts
		g.GET("/mail/unread-counts", func(c *gin.Context) {
			c.JSON(200, gin.H{})
		})

		// Domains
		g.GET("/domains", domainHandler.List)
		g.POST("/domains", domainHandler.Create)
		g.GET("/domains/:id", domainHandler.Get)
		g.DELETE("/domains/:id", domainHandler.Delete)
		g.POST("/domains/:id/verify", domainHandler.Verify)
		g.POST("/domains/:id/dns/regenerate", domainHandler.RegenerateDNS)

		// Email Accounts
		g.GET("/email-accounts", emailHandler.List)
		g.POST("/email-accounts", emailHandler.Create)
		g.PUT("/email-accounts/:id", emailHandler.Update)
		g.DELETE("/email-accounts/:id", emailHandler.Delete)

		// Mail operations
		g.GET("/mail/folders", mailHandler.ListFolders)
		g.GET("/mail/messages", mailHandler.ListMessages)
		g.GET("/mail/messages/:id", mailHandler.GetMessage)
		g.POST("/mail/send", mailHandler.SendMessage)
		g.PUT("/mail/messages/:id/read", mailHandler.MarkRead)
		g.PUT("/mail/messages/:id/unread", mailHandler.MarkUnread)
		g.DELETE("/mail/messages/:id", mailHandler.DeleteMessage)
		g.POST("/mail/messages/:id/move", mailHandler.MoveMessage)

		// Marketing
		g.GET("/marketing/campaigns", marketingHandler.ListCampaigns)
		g.POST("/marketing/campaigns", marketingHandler.CreateCampaign)
		g.GET("/marketing/campaigns/:id", marketingHandler.GetCampaign)
		g.POST("/marketing/campaigns/:id/schedule", marketingHandler.ScheduleCampaign)
		g.POST("/marketing/campaigns/:id/send", marketingHandler.SendCampaign)
		g.GET("/marketing/campaigns/:id/stats", marketingHandler.CampaignStats)
		g.GET("/marketing/lists", marketingHandler.Lists)
		g.POST("/marketing/lists", marketingHandler.CreateList)
		g.POST("/marketing/lists/:listId/subscribers", marketingHandler.AddSubscriber)
		g.POST("/marketing/subscribers/bulk", marketingHandler.BulkAddSubscribers)

		// Packages (authenticated)
		g.PUT("/packages/:id", pkgHandler.Update)
	}

	setupAuthRoutes(v1)
	setupAuthRoutes(apiLegacy)

	// --- Admin routes ---
	setupAdminRoutes := func(g *gin.RouterGroup) {
		g.Use(middleware.AuthMiddleware(&cfg.JWT))
		g.Use(middleware.AdminMiddleware())

		// Dashboard
		g.GET("/dashboard", adminHandler.Dashboard)

		// Users
		g.GET("/users", adminHandler.ListUsers)
		g.PUT("/users/:id", adminHandler.UpdateUser)
		g.POST("/users/:id/suspend", adminHandler.SuspendUser)
		g.POST("/users/:id/activate", adminHandler.ActivateUser)

		// SMTP Configs
		g.GET("/smtp-configs", smtpHandler.List)
		g.POST("/smtp-configs", smtpHandler.Create)
		g.PUT("/smtp-configs/:id", smtpHandler.Update)
		g.DELETE("/smtp-configs/:id", smtpHandler.Delete)
		g.POST("/smtp-configs/:id/assign", smtpHandler.AssignToDomain)
		g.POST("/smtp-configs/:id/test", smtpHandler.TestConnection)

		// Packages
		g.POST("/packages", pkgHandler.Create)

		// DNS Templates
		g.GET("/dns-templates", adminHandler.ListDNSTemplates)
		g.POST("/dns-templates", adminHandler.CreateDNSTemplate)
		g.PUT("/dns-templates/:id", adminHandler.UpdateDNSTemplate)
		g.DELETE("/dns-templates/:id", adminHandler.DeleteDNSTemplate)

		// System Settings
		g.GET("/settings", adminHandler.GetSettings)
		g.PUT("/settings/:key", adminHandler.UpdateSetting)

		// Audit Logs
		g.GET("/audit-logs", adminHandler.GetAuditLogs)
	}

	setupAdminRoutes(v1.Group("/admin"))
	setupAdminRoutes(apiLegacy.Group("/admin"))

	return r
}

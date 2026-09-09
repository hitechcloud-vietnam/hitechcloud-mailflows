package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/api/router"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/config"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/database"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/jmap"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/mcp"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/models"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/smtp"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// @title           HiTechCloud MailFlows API
// @version         1.0
// @description     HiTechCloud MailFlows - Enterprise Email SaaS Platform
// @termsOfService  https://hitechcloud.io/terms

// @contact.name   HiTechCloud Support
// @contact.url    https://hitechcloud.io/support
// @contact.email  support@hitechcloud.io

// @license.name   AGPL-3.0
// @license.url    https://www.gnu.org/licenses/agpl-3.0.html

// @host           localhost:8080
// @BasePath       /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logger
	logger, _ := zap.NewProduction()
	if cfg.Server.Environment == "dev" {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()

	logger.Info("Starting HiTechCloud MailFlows",
		zap.String("version", "1.0.0"),
		zap.String("environment", cfg.Server.Environment),
	)

	// Connect to database
	db, err := database.Connect(&cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	// Run migrations
	if err := database.AutoMigrate(db); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Seed default data
	seedDefaults(db, cfg)

	// Setup HTTP API server
	ginEngine := router.Setup(cfg, db)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      ginEngine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start SMTP server
	smtpServer := smtp.NewServer(&cfg.SMTP, db)
	go func() {
		logger.Info("Starting SMTP server",
			zap.Int("port", cfg.SMTP.Port),
		)
		if err := smtpServer.Start(); err != nil {
			logger.Error("SMTP server error", zap.Error(err))
		}
	}()

	// Start JMAP server
	jmapServer := jmap.NewServer(&cfg.JMAP, db)
	go func() {
		logger.Info("Starting JMAP server",
			zap.Int("port", cfg.JMAP.Port),
		)
		if err := jmapServer.Start(); err != nil {
			logger.Error("JMAP server error", zap.Error(err))
		}
	}()

	// Start MCP server
	if cfg.MCP.Enabled {
		mcpServer := mcp.NewMCPServer(&cfg.MCP, db)
		go func() {
			logger.Info("Starting MCP server",
				zap.Int("port", cfg.MCP.Port),
			)
			if err := mcpServer.Start(); err != nil {
				logger.Error("MCP server error", zap.Error(err))
			}
		}()
	}

	// Start HTTP server
	go func() {
		logger.Info("Starting HTTP API server",
			zap.String("address", httpServer.Addr),
		)
		logger.Info("Swagger UI available",
			zap.String("url", fmt.Sprintf("http://%s/swagger/index.html", httpServer.Addr)),
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// ============================================================
	// Graceful Shutdown
	// ============================================================
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	if err := smtpServer.Stop(); err != nil {
		logger.Error("SMTP server shutdown error", zap.Error(err))
	}

	logger.Info("All servers stopped")
}

func seedDefaults(db *gorm.DB, cfg *config.Config) {
	gormDB := db

	// Seed admin user
	var adminCount int64
	gormDB.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&adminCount)
	if adminCount == 0 && cfg.Admin.DefaultEmail != "" {
		log.Println("Creating default admin user...")
		hash, _ := bcrypt.GenerateFromPassword([]byte(cfg.Admin.DefaultPassword), bcrypt.DefaultCost)
		admin := models.User{
			Email:        cfg.Admin.DefaultEmail,
			PasswordHash: string(hash),
			FirstName:    "Admin",
			LastName:     "User",
			Role:         models.RoleAdmin,
			Status:       models.UserStatusActive,
			IsEmailVerified: true,
		}
		gormDB.Create(&admin)
		log.Printf("Admin user created: %s", cfg.Admin.DefaultEmail)
	}

	// Seed default packages
	var pkgCount int64
	gormDB.Model(&models.Package{}).Count(&pkgCount)
	if pkgCount == 0 {
		log.Println("Creating default packages...")
		seedDefaultPackages(gormDB)
	}

	// Seed default DNS template
	var templateCount int64
	gormDB.Model(&models.DNSTemplate{}).Count(&templateCount)
	if templateCount == 0 {
		log.Println("Creating default DNS template...")
		seedDefaultDNSTemplate(gormDB)
	}
}

func seedDefaultPackages(db *database.DBType) {
	packages := []models.Package{
		{
			Name:          "Free",
			Slug:          "free",
			Description:   "Basic email for personal use",
			PriceMonthly:  0,
			PriceYearly:   0,
			Currency:      "USD",
			IsActive:      true,
			IsPublic:      true,
			SortOrder:     0,
			MaxDomains:    1,
			MaxMailboxes:  1,
			MaxAliases:    3,
			MaxStorageGB:  1,
			MaxSendPerDay: 50,
			MaxRecipients: 100,
			Features: models.PackageFeatures{
				IMAPAccess: true,
				SMTPRelay:  true,
				WebMail:    true,
				AntiSpam:   true,
				AntiVirus:  true,
			},
			FeatureGroups: []string{"email_basic", "security_basic"},
		},
		{
			Name:          "Starter",
			Slug:          "starter",
			Description:   "Perfect for small businesses and freelancers",
			PriceMonthly:  9.99,
			PriceYearly:   99.99,
			Currency:      "USD",
			IsActive:      true,
			IsPublic:      true,
			SortOrder:     1,
			MaxDomains:    3,
			MaxMailboxes:  25,
			MaxAliases:    50,
			MaxStorageGB:  25,
			MaxSendPerDay: 500,
			MaxRecipients: 5000,
			Features: models.PackageFeatures{
				IMAPAccess:      true,
				POP3Access:      true,
				SMTPRelay:       true,
				WebMail:         true,
				MobileSync:      true,
				Contacts:        true,
				Calendar:        true,
				EmailForwarding: true,
				AutoReply:       true,
				EmailAliases:    true,
				AntiSpam:        true,
				AntiVirus:       true,
				DKIMSigning:     true,
				MFA:             true,
				RESTAPI:         true,
				EmailMarketing:  true,
			},
			FeatureGroups: []string{"email_standard", "security_basic", "marketing_lite", "api_basic"},
		},
		{
			Name:          "Professional",
			Slug:          "professional",
			Description:   "For growing businesses that need advanced features",
			PriceMonthly:  29.99,
			PriceYearly:   299.99,
			Currency:      "USD",
			IsActive:      true,
			IsPublic:      true,
			SortOrder:     2,
			MaxDomains:    10,
			MaxMailboxes:  100,
			MaxAliases:    200,
			MaxStorageGB:  100,
			MaxSendPerDay: 2000,
			MaxRecipients: 20000,
			Features: models.PackageFeatures{
				IMAPAccess:        true,
				POP3Access:        true,
				JMAPAccess:        true,
				SMTPRelay:         true,
				WebMail:           true,
				MobileSync:        true,
				Contacts:          true,
				Calendar:          true,
				Tasks:             true,
				EmailForwarding:   true,
				AutoReply:         true,
				CatchAll:          true,
				DistributionLists: true,
				SharedMailboxes:   true,
				EmailAliases:      true,
				AntiSpam:          true,
				AntiVirus:         true,
				Encryption:        true,
				MFA:               true,
				AuditLog:          true,
				DKIMSigning:       true,
				EmailMarketing:    true,
				MarketingTemplates: true,
				MarketingAnalytics: true,
				RESTAPI:           true,
				JMAPAPI:           true,
				Webhooks:          true,
				CustomDNS:         true,
			},
			FeatureGroups: []string{"email_premium", "security_premium", "marketing_pro", "api_premium"},
		},
		{
			Name:          "Enterprise",
			Slug:          "enterprise",
			Description:   "Full-featured with dedicated support and SLA",
			PriceMonthly:  99.99,
			PriceYearly:   999.99,
			Currency:      "USD",
			IsActive:      true,
			IsPublic:      true,
			SortOrder:     3,
			MaxDomains:    -1, // unlimited
			MaxMailboxes:  -1,
			MaxAliases:    -1,
			MaxStorageGB:  -1,
			MaxSendPerDay: -1,
			MaxRecipients: -1,
			Features: models.PackageFeatures{
				IMAPAccess:           true,
				POP3Access:           true,
				JMAPAccess:           true,
				SMTPRelay:            true,
				WebMail:              true,
				MobileSync:           true,
				Contacts:             true,
				Calendar:             true,
				Tasks:                true,
				EmailForwarding:      true,
				AutoReply:            true,
				CatchAll:             true,
				DistributionLists:    true,
				SharedMailboxes:      true,
				EmailAliases:         true,
				AntiSpam:             true,
				AntiVirus:            true,
				Encryption:           true,
				MFA:                  true,
				AuditLog:             true,
				DKIMSigning:          true,
				EmailMarketing:       true,
				MarketingTemplates:   true,
				MarketingAnalytics:   true,
				MarketingAutomation:  true,
				ABTesting:            true,
				RESTAPI:              true,
				JMAPAPI:              true,
				Webhooks:             true,
				CustomDNS:            true,
				WhiteLabel:           true,
				PrioritySupport:      true,
				SLAgreement:          true,
				DedicatedIP:          true,
			},
			FeatureGroups: []string{"email_premium", "security_premium", "marketing_pro", "api_premium", "white_label", "enterprise"},
		},
	}

	for _, pkg := range packages {
		database.DB.Create(&pkg)
	}
}

func seedDefaultDNSTemplate(db *database.DBType) {
	template := models.DNSTemplate{
		Name:        "Standard Email",
		Description: "Standard DNS records for email hosting (MX, SPF, DKIM, DMARC, MTA-STS, TLSRPT, Autoconfig)",
		IsDefault:   true,
	}

	database.DB.Create(&template)

	records := []models.DNSTemplateRecord{
		{TemplateID: template.ID, Type: "MX", Host: "@", Value: "mail.{{domain}}", Priority: intPtr(10), TTL: 3600, SortOrder: 0},
		{TemplateID: template.ID, Type: "A", Host: "mail.{{domain}}", Value: "{{server_ip}}", TTL: 3600, SortOrder: 1},
		{TemplateID: template.ID, Type: "TXT", Host: "@", Value: "v=spf1 mx a:mail.{{domain}} -all", TTL: 3600, SortOrder: 2},
		{TemplateID: template.ID, Type: "TXT", Host: "mailflows._domainkey.{{domain}}", Value: "{{dkim_public_key}}", TTL: 3600, SortOrder: 3},
		{TemplateID: template.ID, Type: "TXT", Host: "_dmarc.{{domain}}", Value: "v=DMARC1; p=quarantine; rua=mailto:dmarc@{{domain}}; pct=100", TTL: 3600, SortOrder: 4},
		{TemplateID: template.ID, Type: "TXT", Host: "_mta-sts.{{domain}}", Value: "v=STSv1; id={{timestamp}}", TTL: 3600, SortOrder: 5},
		{TemplateID: template.ID, Type: "TXT", Host: "_smtp._tls.{{domain}}", Value: "v=TLSRPTv1; rua=mailto:tlsrpt@{{domain}}", TTL: 3600, SortOrder: 6},
		{TemplateID: template.ID, Type: "CNAME", Host: "autoconfig.{{domain}}", Value: "mail.{{domain}}", TTL: 3600, SortOrder: 7},
		{TemplateID: template.ID, Type: "CNAME", Host: "autodiscover.{{domain}}", Value: "mail.{{domain}}", TTL: 3600, SortOrder: 8},
		{TemplateID: template.ID, Type: "SRV", Host: "_imaps._tcp.{{domain}}", Value: "0 1 993 mail.{{domain}}", TTL: 3600, SortOrder: 9},
		{TemplateID: template.ID, Type: "SRV", Host: "_submissions._tcp.{{domain}}", Value: "0 1 587 mail.{{domain}}", TTL: 3600, SortOrder: 10},
		{TemplateID: template.ID, Type: "SRV", Host: "_jmap._tcp.{{domain}}", Value: "0 1 443 mail.{{domain}}", TTL: 3600, SortOrder: 11},
	}

	for _, r := range records {
		database.DB.Create(&r)
	}
}

func intPtr(i int) *int {
	return &i
}

package database

import (
	"fmt"
	"log"

	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/config"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// DBType is an alias for gorm.DB used in type references
type DBType = gorm.DB

func Connect(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() interface{} {
			return gorm.Expr("NOW()")
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	log.Println("Database connected successfully")
	return DB, nil
}

func AutoMigrate(db *gorm.DB) error {
	log.Println("Running database migrations...")

	err := db.AutoMigrate(
		// Users & Auth
		&models.User{},
		&models.RefreshToken{},
		&models.PasswordResetToken{},
		&models.AuthEvent{},

		// Domains & DNS
		&models.Domain{},
		&models.DNSRecord{},
		&models.DNSTemplate{},
		&models.DNSTemplateRecord{},

		// SMTP
		&models.SMTPConfig{},

		// Email Accounts
		&models.EmailAccount{},

		// Packages
		&models.Package{},

		// Marketing
		&models.MarketingCampaign{},
		&models.MarketingList{},
		&models.MarketingSubscriber{},
		&models.CampaignEvent{},

		// System
		&models.SystemSetting{},
		&models.AuditLog{},
	)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations completed")
	return nil
}

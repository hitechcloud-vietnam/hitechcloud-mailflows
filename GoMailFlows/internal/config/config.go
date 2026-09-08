package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	SMTP      SMTPConfig      `mapstructure:"smtp"`
	IMAP      IMAPConfig      `mapstructure:"imap"`
	JMAP      JMAPConfig      `mapstructure:"jmap"`
	Admin     AdminConfig     `mapstructure:"admin"`
	Marketing MarketingConfig `mapstructure:"marketing"`
	TLS       TLSConfig       `mapstructure:"tls"`
	MCP       MCPConfig       `mapstructure:"mcp"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	Environment     string        `mapstructure:"environment"` // dev, staging, production
	BaseURL         string        `mapstructure:"base_url"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Name            string        `mapstructure:"name"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret            string        `mapstructure:"secret"`
	AccessTokenTTL    time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL   time.Duration `mapstructure:"refresh_token_ttl"`
	Issuer            string        `mapstructure:"issuer"`
}

type SMTPConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	TLSPort        int    `mapstructure:"tls_port"`
	MaxConnections int    `mapstructure:"max_connections"`
	MaxMessageSize int64  `mapstructure:"max_message_size"` // bytes
	AuthRequired   bool   `mapstructure:"auth_required"`
}

type IMAPConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	TLSPort        int    `mapstructure:"tls_port"`
	MaxConnections int    `mapstructure:"max_connections"`
}

type JMAPConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type AdminConfig struct {
	DefaultEmail    string `mapstructure:"default_email"`
	DefaultPassword string `mapstructure:"default_password"`
}

type MarketingConfig struct {
	MaxRecipientsPerCampaign int    `mapstructure:"max_recipients_per_campaign"`
	UnsubscribeBaseURL       string `mapstructure:"unsubscribe_base_url"`
	BounceThreshold          int    `mapstructure:"bounce_threshold"`
}

type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
	ACME     bool   `mapstructure:"acme"` // Let's Encrypt auto
	Domains  []string `mapstructure:"domains"`
}

type MCPConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/mailflows")

	// Defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.shutdown_timeout", "10s")
	viper.SetDefault("server.environment", "dev")

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "mailflows")
	viper.SetDefault("database.password", "mailflows")
	viper.SetDefault("database.name", "mailflows")
	viper.SetDefault("database.ssl_mode", "disable")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.conn_max_lifetime", "5m")

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)

	viper.SetDefault("jwt.access_token_ttl", "15m")
	viper.SetDefault("jwt.refresh_token_ttl", "168h") // 7 days
	viper.SetDefault("jwt.issuer", "mailflows")

	viper.SetDefault("smtp.host", "0.0.0.0")
	viper.SetDefault("smtp.port", 25)
	viper.SetDefault("smtp.tls_port", 587)
	viper.SetDefault("smtp.max_connections", 100)
	viper.SetDefault("smtp.max_message_size", 26214400) // 25MB
	viper.SetDefault("smtp.auth_required", true)

	viper.SetDefault("imap.host", "0.0.0.0")
	viper.SetDefault("imap.port", 143)
	viper.SetDefault("imap.tls_port", 993)
	viper.SetDefault("imap.max_connections", 100)

	viper.SetDefault("jmap.host", "0.0.0.0")
	viper.SetDefault("jmap.port", 8081)

	viper.SetDefault("marketing.max_recipients_per_campaign", 10000)
	viper.SetDefault("marketing.bounce_threshold", 5)

	viper.SetDefault("mcp.enabled", true)
	viper.SetDefault("mcp.host", "0.0.0.0")
	viper.SetDefault("mcp.port", 8082)

	// Environment variable override
	viper.AutomaticEnv()
	viper.SetEnvPrefix("MF")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================
// User & Auth Models
// ============================================================

type User struct {
	ID                uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Email             string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email" validate:"required,email"`
	PasswordHash      string         `gorm:"type:varchar(255);not null" json:"-"`
	FirstName         string         `gorm:"type:varchar(100)" json:"first_name"`
	LastName          string         `gorm:"type:varchar(100)" json:"last_name"`
	Role              UserRole       `gorm:"type:varchar(20);default:'user';not null" json:"role"`
	Status            UserStatus     `gorm:"type:varchar(20);default:'active';not null" json:"status"`
	IsEmailVerified   bool           `gorm:"default:false" json:"is_email_verified"`
	MFAEnabled        bool           `gorm:"default:false" json:"mfa_enabled"`
	MFASecret         string         `gorm:"type:varchar(255)" json:"-"`
	MFABackupCodes    string         `gorm:"type:text" json:"-"`
	LastLoginAt       *time.Time     `json:"last_login_at"`
	LastLoginIP       string         `gorm:"type:varchar(45)" json:"-"`
	PackageID         *uuid.UUID     `gorm:"type:uuid" json:"package_id"`
	Package           *Package       `gorm:"foreignKey:PackageID" json:"package,omitempty"`
	StorageUsed       int64          `gorm:"default:0" json:"storage_used"`       // bytes
	StorageQuota      int64          `gorm:"default:0" json:"storage_quota"`       // bytes, 0 = unlimited
	MaxEmailAccounts  int            `gorm:"default:1" json:"max_email_accounts"`
	EmailAccountCount int            `gorm:"default:0" json:"email_account_count"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleReseller UserRole = "reseller"
	RoleUser     UserRole = "user"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusPending   UserStatus = "pending"
	UserStatusBanned    UserStatus = "banned"
)

type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	TokenHash string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	Revoked   bool      `gorm:"default:false"`
	UserAgent string    `gorm:"type:varchar(500)"`
	IPAddress string    `gorm:"type:varchar(45)"`
	CreatedAt time.Time
}

type PasswordResetToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;index;not null"`
	TokenHash string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	Used      bool      `gorm:"default:false"`
	CreatedAt time.Time
}

type AuthEvent struct {
	ID        uuid.UUID    `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    *uuid.UUID   `gorm:"type:uuid;index"`
	Event     AuthEventType `gorm:"type:varchar(50);not null"`
	IPAddress string       `gorm:"type:varchar(45)"`
	UserAgent string       `gorm:"type:varchar(500)"`
	Success   bool         `gorm:"not null"`
	Details   string       `gorm:"type:text"`
	CreatedAt time.Time
}

type AuthEventType string

const (
	AuthEventLogin         AuthEventType = "login"
	AuthEventLogout        AuthEventType = "logout"
	AuthEventRegister      AuthEventType = "register"
	AuthEventPasswordReset AuthEventType = "password_reset"
	AuthEventMFAEnabled    AuthEventType = "mfa_enabled"
	AuthEventMFADisabled   AuthEventType = "mfa_disabled"
	AuthEventTokenRefresh  AuthEventType = "token_refresh"
	AuthEventPasswordChange AuthEventType = "password_change"
)

// ============================================================
// Domain & DNS Models
// ============================================================

type Domain struct {
	ID                uuid.UUID       `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID           uuid.UUID       `gorm:"type:uuid;index;not null" json:"owner_id"`
	Owner             *User           `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Name              string          `gorm:"type:varchar(255);uniqueIndex;not null" json:"name" validate:"required,fqdn"`
	Status            DomainStatus    `gorm:"type:varchar(20);default:'pending';not null" json:"status"`
	VerificationToken string          `gorm:"type:varchar(255)" json:"-"`
	VerifiedAt        *time.Time      `json:"verified_at"`
	MXVerified        bool            `gorm:"default:false" json:"mx_verified"`
	SPFVerified       bool            `gorm:"default:false" json:"spf_verified"`
	DKIMVerified      bool            `gorm:"default:false" json:"dkim_verified"`
	DMARCVerified     bool            `gorm:"default:false" json:"dmarc_verified"`
	MtaSTSVerified    bool            `gorm:"default:false" json:"mta_sts_verified"`
	TLSRPTVerified    bool            `gorm:"default:false" json:"tls_rpt_verified"`
	DKIMSelector      string          `gorm:"type:varchar(63);default:'mailflows'" json:"dkim_selector"`
	DKIMPublicKey     string          `gorm:"type:text" json:"dkim_public_key,omitempty"`
	DKIMPrivateKey    string          `gorm:"type:text" json:"-"`
	MaxMailboxes      int             `gorm:"default:10" json:"max_mailboxes"`
	MailboxCount      int             `gorm:"default:0" json:"mailbox_count"`
	CatchAllAddress   string          `gorm:"type:varchar(255)" json:"catch_all_address"`
	SMTPConfigID      *uuid.UUID      `gorm:"type:uuid" json:"smtp_config_id"`
	SMTPConfig        *SMTPConfig     `gorm:"foreignKey:SMTPConfigID" json:"smtp_config,omitempty"`
	TemplateID        *uuid.UUID      `gorm:"type:uuid" json:"template_id"`
	Template          *DNSTemplate    `gorm:"foreignKey:TemplateID" json:"template,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	DeletedAt         gorm.DeletedAt  `gorm:"index" json:"-"`
}

type DomainStatus string

const (
	DomainStatusPending    DomainStatus = "pending"
	DomainStatusVerifying  DomainStatus = "verifying"
	DomainStatusActive     DomainStatus = "active"
	DomainStatusFailed     DomainStatus = "failed"
	DomainStatusSuspended  DomainStatus = "suspended"
)

type DNSRecord struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	DomainID  uuid.UUID  `gorm:"type:uuid;index;not null" json:"domain_id"`
	Type      DNSRecordType `gorm:"type:varchar(10);not null" json:"type" validate:"required"`
	Host      string     `gorm:"type:varchar(255);not null" json:"host" validate:"required"`
	Value     string     `gorm:"type:text;not null" json:"value" validate:"required"`
	Priority  *int       `gorm:"type:int" json:"priority,omitempty"`
	TTL       int        `gorm:"default:3600" json:"ttl"`
	IsManaged bool       `gorm:"default:true" json:"is_managed"` // auto-generated by system
	Verified  bool       `gorm:"default:false" json:"verified"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type DNSRecordType string

const (
	DNSRecordMX    DNSRecordType = "MX"
	DNSRecordA     DNSRecordType = "A"
	DNSRecordAAAA  DNSRecordType = "AAAA"
	DNSRecordCNAME DNSRecordType = "CNAME"
	DNSRecordTXT   DNSRecordType = "TXT"
	DNSRecordSRV   DNSRecordType = "SRV"
	DNSRecordCAA   DNSRecordType = "CAA"
	DNSRecordNS    DNSRecordType = "NS"
)

type DNSTemplate struct {
	ID          uuid.UUID           `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name        string              `gorm:"type:varchar(100);not null" json:"name" validate:"required"`
	Description string              `gorm:"type:text" json:"description"`
	IsDefault   bool                `gorm:"default:false" json:"is_default"`
	Records     []DNSTemplateRecord `gorm:"foreignKey:TemplateID" json:"records"`
	CreatedBy   uuid.UUID           `gorm:"type:uuid" json:"created_by"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

type DNSTemplateRecord struct {
	ID         uuid.UUID     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TemplateID uuid.UUID     `gorm:"type:uuid;index;not null" json:"template_id"`
	Type       DNSRecordType `gorm:"type:varchar(10);not null" json:"type"`
	Host       string        `gorm:"type:varchar(255);not null" json:"host"` // supports {{domain}} placeholder
	Value      string        `gorm:"type:text;not null" json:"value"`        // supports placeholders
	Priority   *int          `gorm:"type:int" json:"priority,omitempty"`
	TTL        int           `gorm:"default:3600" json:"ttl"`
	SortOrder  int           `gorm:"default:0" json:"sort_order"`
}

// ============================================================
// SMTP Configuration Models
// ============================================================

type SMTPConfig struct {
	ID            uuid.UUID    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name          string       `gorm:"type:varchar(100);not null" json:"name" validate:"required"`
	Host          string       `gorm:"type:varchar(255);not null" json:"host" validate:"required"`
	Port          int          `gorm:"not null" json:"port" validate:"required"`
	TLSPort       int          `json:"tls_port"`
	Username      string       `gorm:"type:varchar(255)" json:"username"`
	Password      string       `gorm:"type:varchar(255)" json:"-"`
	AuthType      SMTPAuthType `gorm:"type:varchar(20);default:'plain'" json:"auth_type"`
	FromDomain    string       `gorm:"type:varchar(255)" json:"from_domain"`
	MaxPerHour    int          `gorm:"default:100" json:"max_per_hour"`
	MaxPerDay     int          `gorm:"default:1000" json:"max_per_day"`
	IsGlobal      bool         `gorm:"default:false" json:"is_global"` // applies to all domains
	IsActive      bool         `gorm:"default:true" json:"is_active"`
	Priority      int          `gorm:"default:0" json:"priority"`      // higher = preferred
	Domains       []Domain     `gorm:"foreignKey:SMTPConfigID" json:"domains,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type SMTPAuthType string

const (
	SMTPAuthPlain    SMTPAuthType = "plain"
	SMTPAuthLogin    SMTPAuthType = "login"
	SMTPAuthCRAMMD5  SMTPAuthType = "cram_md5"
	SMTPAuthOAuth2   SMTPAuthType = "oauth2"
	SMTPAuthNone     SMTPAuthType = "none"
)

// ============================================================
// Email Account (Mailbox) Models
// ============================================================

type EmailAccount struct {
	ID              uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID          uuid.UUID      `gorm:"type:uuid;index;not null" json:"user_id"`
	User            *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	DomainID        uuid.UUID      `gorm:"type:uuid;index;not null" json:"domain_id"`
	Domain          *Domain        `gorm:"foreignKey:DomainID" json:"domain,omitempty"`
	Address         string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"address" validate:"required,email"`
	PasswordHash    string         `gorm:"type:varchar(255);not null" json:"-"`
	DisplayName     string         `gorm:"type:varchar(255)" json:"display_name"`
	IsEnabled       bool           `gorm:"default:true" json:"is_enabled"`
	IsAdmin         bool           `gorm:"default:false" json:"is_admin"` // domain admin
	StorageUsed     int64          `gorm:"default:0" json:"storage_used"`
	StorageQuota    int64          `gorm:"default:1073741824" json:"storage_quota"` // 1GB default
	MaxSendPerDay   int            `gorm:"default:500" json:"max_send_per_day"`
	SentToday       int            `gorm:"default:0" json:"sent_today"`
	ForwardTo       string         `gorm:"type:varchar(255)" json:"forward_to"`
	AutoReply       bool           `gorm:"default:false" json:"auto_reply"`
	AutoReplyText   string         `gorm:"type:text" json:"auto_reply_text"`
	LastLoginAt     *time.Time     `json:"last_login_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// ============================================================
// SaaS Package Models
// ============================================================

type Package struct {
	ID               uuid.UUID          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name             string             `gorm:"type:varchar(100);not null;uniqueIndex" json:"name" validate:"required"`
	Slug             string             `gorm:"type:varchar(100);not null;uniqueIndex" json:"slug"`
	Description      string             `gorm:"type:text" json:"description"`
	PriceMonthly     float64            `gorm:"default:0" json:"price_monthly"`
	PriceYearly      float64            `gorm:"default:0" json:"price_yearly"`
	Currency         string             `gorm:"type:varchar(3);default:'USD'" json:"currency"`
	IsActive         bool               `gorm:"default:true" json:"is_active"`
	IsPublic         bool               `gorm:"default:true" json:"is_public"` // shown on pricing page
	SortOrder        int                `gorm:"default:0" json:"sort_order"`
	MaxDomains       int                `gorm:"default:1" json:"max_domains"`
	MaxMailboxes     int                `gorm:"default:10" json:"max_mailboxes"`
	MaxAliases       int                `gorm:"default:20" json:"max_aliases"`
	MaxStorageGB     int                `gorm:"default:5" json:"max_storage_gb"`
	MaxSendPerDay    int                `gorm:"default:500" json:"max_send_per_day"`
	MaxRecipients    int                `gorm:"default:1000" json:"max_recipients"`
	Features         PackageFeatures    `gorm:"type:jsonb" json:"features"`
	FeatureGroups    []string           `gorm:"type:jsonb" json:"feature_groups"` // e.g. ["email_basic", "marketing_lite"]
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

type PackageFeatures struct {
	// Core Email
	IMAPAccess       bool `json:"imap_access"`
	POP3Access       bool `json:"pop3_access"`
	JMAPAccess       bool `json:"jmap_access"`
	SMTPRelay        bool `json:"smtp_relay"`
	WebMail          bool `json:"webmail"`
	MobileSync       bool `json:"mobile_sync"`       // ActiveSync/EAS

	// Advanced Email
	Calendar         bool `json:"calendar"`
	Contacts         bool `json:"contacts"`
	Tasks            bool `json:"tasks"`
	EmailForwarding  bool `json:"email_forwarding"`
	AutoReply        bool `json:"auto_reply"`
	CatchAll         bool `json:"catch_all"`
	DistributionLists bool `json:"distribution_lists"`
	SharedMailboxes  bool `json:"shared_mailboxes"`
	EmailAliases     bool `json:"email_aliases"`

	// Security
	AntiSpam         bool `json:"anti_spam"`
	AntiVirus        bool `json:"anti_virus"`
	Encryption       bool `json:"encryption"`       // S/MIME, PGP
	MFA              bool `json:"mfa"`
	AuditLog         bool `json:"audit_log"`
	DKIMSigning      bool `json:"dkim_signing"`

	// Marketing
	EmailMarketing   bool `json:"email_marketing"`
	MarketingTemplates bool `json:"marketing_templates"`
	MarketingAnalytics bool `json:"marketing_analytics"`
	MarketingAutomation bool `json:"marketing_automation"`
	ABTesting        bool `json:"ab_testing"`

	// API & Integration
	RESTAPI          bool `json:"rest_api"`
	JMAPAPI          bool `json:"jmap_api"`
	Webhooks         bool `json:"webhooks"`
	CustomDNS        bool `json:"custom_dns"`
	WhiteLabel       bool `json:"white_label"`

	// Support
	PrioritySupport  bool `json:"priority_support"`
	SLAgreement      bool `json:"sla_agreement"`
	DedicatedIP      bool `json:"dedicated_ip"`
}

// ============================================================
// Email Marketing Models
// ============================================================

type MarketingCampaign struct {
	ID            uuid.UUID          `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID          `gorm:"type:uuid;index;not null" json:"user_id"`
	Name          string             `gorm:"type:varchar(255);not null" json:"name" validate:"required"`
	Subject       string             `gorm:"type:varchar(500);not null" json:"subject" validate:"required"`
	FromName      string             `gorm:"type:varchar(255)" json:"from_name"`
	FromEmail     string             `gorm:"type:varchar(255);not null" json:"from_email" validate:"required,email"`
	ReplyTo       string             `gorm:"type:varchar(255)" json:"reply_to"`
	HTMLContent   string             `gorm:"type:text" json:"html_content"`
	PlainText     string             `gorm:"type:text" json:"plain_text"`
	Status        CampaignStatus     `gorm:"type:varchar(20);default:'draft'" json:"status"`
	ScheduledAt   *time.Time         `json:"scheduled_at"`
	SentAt        *time.Time         `json:"sent_at"`
	TotalRecipients int              `gorm:"default:0" json:"total_recipients"`
	SentCount     int                `gorm:"default:0" json:"sent_count"`
	OpenCount     int                `gorm:"default:0" json:"open_count"`
	ClickCount    int                `gorm:"default:0" json:"click_count"`
	BounceCount   int                `gorm:"default:0" json:"bounce_count"`
	UnsubCount    int                `gorm:"default:0" json:"unsub_count"`
	SpamCount     int                `gorm:"default:0" json:"spam_count"`
	Tags          []string           `gorm:"type:jsonb" json:"tags"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

type CampaignStatus string

const (
	CampaignStatusDraft     CampaignStatus = "draft"
	CampaignStatusScheduled CampaignStatus = "scheduled"
	CampaignStatusSending   CampaignStatus = "sending"
	CampaignStatusSent      CampaignStatus = "sent"
	CampaignStatusPaused    CampaignStatus = "paused"
	CampaignStatusCancelled CampaignStatus = "cancelled"
)

type MarketingList struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;index;not null" json:"user_id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	SubscriberCount int        `gorm:"default:0" json:"subscriber_count"`
	Tags        []string       `gorm:"type:jsonb" json:"tags"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type MarketingSubscriber struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ListID      uuid.UUID      `gorm:"type:uuid;index;not null" json:"list_id"`
	Email       string         `gorm:"type:varchar(255);not null" json:"email" validate:"required,email"`
	FirstName   string         `gorm:"type:varchar(100)" json:"first_name"`
	LastName    string         `gorm:"type:varchar(100)" json:"last_name"`
	Status      SubscriberStatus `gorm:"type:varchar(20);default:'active'" json:"status"`
	MetaData    map[string]string `gorm:"type:jsonb" json:"meta_data,omitempty"`
	ConfirmedAt *time.Time     `json:"confirmed_at"`
	UnsubscribedAt *time.Time  `json:"unsubscribed_at"`
	BounceCount int            `gorm:"default:0" json:"bounce_count"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type SubscriberStatus string

const (
	SubscriberStatusActive      SubscriberStatus = "active"
	SubscriberStatusUnconfirmed SubscriberStatus = "unconfirmed"
	SubscriberStatusUnsubscribed SubscriberStatus = "unsubscribed"
	SubscriberStatusBounced     SubscriberStatus = "bounced"
	SubscriberStatusComplained  SubscriberStatus = "complained"
)

type CampaignEvent struct {
	ID         uuid.UUID    `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CampaignID uuid.UUID    `gorm:"type:uuid;index;not null" json:"campaign_id"`
	SubscriberID uuid.UUID  `gorm:"type:uuid;index;not null" json:"subscriber_id"`
	EventType  CampaignEventType `gorm:"type:varchar(20);not null" json:"event_type"`
	IPAddress  string       `gorm:"type:varchar(45)" json:"-"`
	UserAgent  string       `gorm:"type:varchar(500)" json:"-"`
	URL        string       `gorm:"type:text" json:"url,omitempty"` // for click events
	CreatedAt  time.Time    `json:"created_at"`
}

type CampaignEventType string

const (
	CampaignEventSent     CampaignEventType = "sent"
	CampaignEventDelivered CampaignEventType = "delivered"
	CampaignEventOpened   CampaignEventType = "opened"
	CampaignEventClicked  CampaignEventType = "clicked"
	CampaignEventBounced  CampaignEventType = "bounced"
	CampaignEventUnsub    CampaignEventType = "unsubscribed"
	CampaignEventComplained CampaignEventType = "complained"
)

// ============================================================
// System & Settings Models
// ============================================================

type SystemSetting struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Key       string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	Category  string    `gorm:"type:varchar(50)" json:"category"`
	IsSecret  bool      `gorm:"default:false" json:"is_secret"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AuditLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID    *uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Action    string    `gorm:"type:varchar(100);not null" json:"action"`
	Resource  string    `gorm:"type:varchar(100);not null" json:"resource"`
	ResourceID string   `gorm:"type:varchar(100)" json:"resource_id"`
	Details   string    `gorm:"type:jsonb" json:"details"`
	IPAddress string    `gorm:"type:varchar(45)" json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
}

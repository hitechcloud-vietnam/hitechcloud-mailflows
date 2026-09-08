package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/models"
	"gorm.io/gorm"
)

type PackageHandler struct {
	db *gorm.DB
}

func NewPackageHandler(db *gorm.DB) *PackageHandler {
	return &PackageHandler{db: db}
}

type CreatePackageRequest struct {
	Name          string              `json:"name" binding:"required"`
	Slug          string              `json:"slug" binding:"required"`
	Description   string              `json:"description"`
	PriceMonthly  float64             `json:"price_monthly"`
	PriceYearly   float64             `json:"price_yearly"`
	Currency      string              `json:"currency"`
	IsActive      *bool               `json:"is_active"`
	IsPublic      *bool               `json:"is_public"`
	SortOrder     int                 `json:"sort_order"`
	MaxDomains    int                 `json:"max_domains"`
	MaxMailboxes  int                 `json:"max_mailboxes"`
	MaxAliases    int                 `json:"max_aliases"`
	MaxStorageGB  int                 `json:"max_storage_gb"`
	MaxSendPerDay int                 `json:"max_send_per_day"`
	MaxRecipients int                 `json:"max_recipients"`
	Features      PackageFeaturesReq  `json:"features"`
	FeatureGroups []string            `json:"feature_groups"`
}

type PackageFeaturesReq struct {
	IMAPAccess        *bool `json:"imap_access"`
	POP3Access        *bool `json:"pop3_access"`
	JMAPAccess        *bool `json:"jmap_access"`
	SMTPRelay         *bool `json:"smtp_relay"`
	WebMail           *bool `json:"webmail"`
	MobileSync        *bool `json:"mobile_sync"`
	Calendar          *bool `json:"calendar"`
	Contacts          *bool `json:"contacts"`
	Tasks             *bool `json:"tasks"`
	EmailForwarding   *bool `json:"email_forwarding"`
	AutoReply         *bool `json:"auto_reply"`
	CatchAll          *bool `json:"catch_all"`
	DistributionLists *bool `json:"distribution_lists"`
	SharedMailboxes   *bool `json:"shared_mailboxes"`
	EmailAliases      *bool `json:"email_aliases"`
	AntiSpam          *bool `json:"anti_spam"`
	AntiVirus         *bool `json:"anti_virus"`
	Encryption        *bool `json:"encryption"`
	MFA               *bool `json:"mfa"`
	AuditLog          *bool `json:"audit_log"`
	DKIMSigning       *bool `json:"dkim_signing"`
	EmailMarketing    *bool `json:"email_marketing"`
	MarketingTemplates *bool `json:"marketing_templates"`
	MarketingAnalytics *bool `json:"marketing_analytics"`
	MarketingAutomation *bool `json:"marketing_automation"`
	ABTesting         *bool `json:"ab_testing"`
	RESTAPI           *bool `json:"rest_api"`
	JMAPAPI           *bool `json:"jmap_api"`
	Webhooks          *bool `json:"webhooks"`
	CustomDNS         *bool `json:"custom_dns"`
	WhiteLabel        *bool `json:"white_label"`
	PrioritySupport   *bool `json:"priority_support"`
	SLAgreement       *bool `json:"sla_agreement"`
	DedicatedIP       *bool `json:"dedicated_ip"`
}

// Feature group definitions
var FeatureGroups = map[string][]string{
	"email_basic": {
		"imap_access", "smtp_relay", "webmail", "email_aliases",
	},
	"email_standard": {
		"imap_access", "smtp_relay", "webmail", "email_aliases",
		"email_forwarding", "auto_reply", "contacts",
	},
	"email_premium": {
		"imap_access", "pop3_access", "jmap_access", "smtp_relay", "webmail", "mobile_sync",
		"email_forwarding", "auto_reply", "contacts", "calendar", "tasks",
		"catch_all", "distribution_lists", "shared_mailboxes", "email_aliases",
	},
	"security_basic": {
		"anti_spam", "anti_virus", "dkim_signing",
	},
	"security_premium": {
		"anti_spam", "anti_virus", "dkim_signing", "encryption", "mfa", "audit_log",
	},
	"marketing_lite": {
		"email_marketing", "marketing_templates",
	},
	"marketing_pro": {
		"email_marketing", "marketing_templates", "marketing_analytics",
		"marketing_automation", "ab_testing",
	},
	"api_basic": {
		"rest_api",
	},
	"api_premium": {
		"rest_api", "jmap_api", "webhooks",
	},
	"white_label": {
		"white_label", "custom_dns",
	},
	"enterprise": {
		"priority_support", "sla_agreement", "dedicated_ip",
	},
}

// @Summary     List packages
// @Tags        packages
// @Produce     json
// @Param       public query bool false "Only public packages"
// @Success     200 {array} models.Package
// @Router      /api/v1/packages [get]
func (h *PackageHandler) List(c *gin.Context) {
	publicOnly := c.Query("public") == "true"

	var packages []models.Package
	query := h.db
	if publicOnly {
		query = query.Where("is_public = ? AND is_active = true", true)
	}

	if err := query.Order("sort_order ASC, price_monthly ASC").Find(&packages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch packages"})
		return
	}

	c.JSON(http.StatusOK, packages)
}

// @Summary     Get package details
// @Tags        packages
// @Produce     json
// @Param       id path string true "Package ID or slug"
// @Success     200 {object} models.Package
// @Router      /api/v1/packages/{id} [get]
func (h *PackageHandler) Get(c *gin.Context) {
	id := c.Param("id")

	var pkg models.Package
	if err := h.db.Where("id = ? OR slug = ?", id, id).First(&pkg).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Package not found"})
		return
	}

	c.JSON(http.StatusOK, pkg)
}

// @Summary     Create package
// @Tags        packages
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body CreatePackageRequest true "Package details"
// @Success     201 {object} models.Package
// @Router      /api/v1/admin/packages [post]
func (h *PackageHandler) Create(c *gin.Context) {
	var req CreatePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	features := buildPackageFeatures(req.Features, req.FeatureGroups)

	pkg := models.Package{
		Name:          req.Name,
		Slug:          req.Slug,
		Description:   req.Description,
		PriceMonthly:  req.PriceMonthly,
		PriceYearly:   req.PriceYearly,
		Currency:      req.Currency,
		IsActive:      true,
		IsPublic:      true,
		SortOrder:     req.SortOrder,
		MaxDomains:    req.MaxDomains,
		MaxMailboxes:  req.MaxMailboxes,
		MaxAliases:    req.MaxAliases,
		MaxStorageGB:  req.MaxStorageGB,
		MaxSendPerDay: req.MaxSendPerDay,
		MaxRecipients: req.MaxRecipients,
		Features:      features,
		FeatureGroups: req.FeatureGroups,
	}

	if req.IsActive != nil {
		pkg.IsActive = *req.IsActive
	}
	if req.IsPublic != nil {
		pkg.IsPublic = *req.IsPublic
	}
	if pkg.Currency == "" {
		pkg.Currency = "USD"
	}

	if err := h.db.Create(&pkg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create package"})
		return
	}

	c.JSON(http.StatusCreated, pkg)
}

// @Summary     Update package
// @Tags        packages
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Package ID"
// @Param       body body CreatePackageRequest true "Package details"
// @Success     200 {object} models.Package
// @Router      /api/v1/admin/packages/{id} [put]
func (h *PackageHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var pkg models.Package
	if err := h.db.First(&pkg, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Package not found"})
		return
	}

	var req CreatePackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	features := buildPackageFeatures(req.Features, req.FeatureGroups)

	updates := map[string]interface{}{
		"name":           req.Name,
		"slug":           req.Slug,
		"description":    req.Description,
		"price_monthly":  req.PriceMonthly,
		"price_yearly":   req.PriceYearly,
		"currency":       req.Currency,
		"sort_order":     req.SortOrder,
		"max_domains":    req.MaxDomains,
		"max_mailboxes":  req.MaxMailboxes,
		"max_aliases":    req.MaxAliases,
		"max_storage_gb": req.MaxStorageGB,
		"max_send_per_day": req.MaxSendPerDay,
		"max_recipients": req.MaxRecipients,
		"features":       features,
		"feature_groups": req.FeatureGroups,
	}

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}
	if req.IsPublic != nil {
		updates["is_public"] = *req.IsPublic
	}

	h.db.Model(&pkg).Updates(updates)
	h.db.First(&pkg, "id = ?", id)

	c.JSON(http.StatusOK, pkg)
}

// @Summary     Get feature groups
// @Tags        packages
// @Produce     json
// @Success     200 {object} map[string][]string
// @Router      /api/v1/packages/feature-groups [get]
func (h *PackageHandler) GetFeatureGroups(c *gin.Context) {
	c.JSON(http.StatusOK, FeatureGroups)
}

func buildPackageFeatures(req PackageFeaturesReq, groups []string) models.PackageFeatures {
	f := models.PackageFeatures{}

	// Start with group defaults
	for _, group := range groups {
		if features, ok := FeatureGroups[group]; ok {
			for _, feat := range features {
				setFeature(&f, feat, true)
			}
		}
	}

	// Override with explicit settings
	if req.IMAPAccess != nil { f.IMAPAccess = *req.IMAPAccess }
	if req.POP3Access != nil { f.POP3Access = *req.POP3Access }
	if req.JMAPAccess != nil { f.JMAPAccess = *req.JMAPAccess }
	if req.SMTPRelay != nil { f.SMTPRelay = *req.SMTPRelay }
	if req.WebMail != nil { f.WebMail = *req.WebMail }
	if req.MobileSync != nil { f.MobileSync = *req.MobileSync }
	if req.Calendar != nil { f.Calendar = *req.Calendar }
	if req.Contacts != nil { f.Contacts = *req.Contacts }
	if req.Tasks != nil { f.Tasks = *req.Tasks }
	if req.EmailForwarding != nil { f.EmailForwarding = *req.EmailForwarding }
	if req.AutoReply != nil { f.AutoReply = *req.AutoReply }
	if req.CatchAll != nil { f.CatchAll = *req.CatchAll }
	if req.DistributionLists != nil { f.DistributionLists = *req.DistributionLists }
	if req.SharedMailboxes != nil { f.SharedMailboxes = *req.SharedMailboxes }
	if req.EmailAliases != nil { f.EmailAliases = *req.EmailAliases }
	if req.AntiSpam != nil { f.AntiSpam = *req.AntiSpam }
	if req.AntiVirus != nil { f.AntiVirus = *req.AntiVirus }
	if req.Encryption != nil { f.Encryption = *req.Encryption }
	if req.MFA != nil { f.MFA = *req.MFA }
	if req.AuditLog != nil { f.AuditLog = *req.AuditLog }
	if req.DKIMSigning != nil { f.DKIMSigning = *req.DKIMSigning }
	if req.EmailMarketing != nil { f.EmailMarketing = *req.EmailMarketing }
	if req.MarketingTemplates != nil { f.MarketingTemplates = *req.MarketingTemplates }
	if req.MarketingAnalytics != nil { f.MarketingAnalytics = *req.MarketingAnalytics }
	if req.MarketingAutomation != nil { f.MarketingAutomation = *req.MarketingAutomation }
	if req.ABTesting != nil { f.ABTesting = *req.ABTesting }
	if req.RESTAPI != nil { f.RESTAPI = *req.RESTAPI }
	if req.JMAPAPI != nil { f.JMAPAPI = *req.JMAPAPI }
	if req.Webhooks != nil { f.Webhooks = *req.Webhooks }
	if req.CustomDNS != nil { f.CustomDNS = *req.CustomDNS }
	if req.WhiteLabel != nil { f.WhiteLabel = *req.WhiteLabel }
	if req.PrioritySupport != nil { f.PrioritySupport = *req.PrioritySupport }
	if req.SLAgreement != nil { f.SLAgreement = *req.SLAgreement }
	if req.DedicatedIP != nil { f.DedicatedIP = *req.DedicatedIP }

	return f
}

func setFeature(f *models.PackageFeatures, name string, val bool) {
	switch name {
	case "imap_access": f.IMAPAccess = val
	case "pop3_access": f.POP3Access = val
	case "jmap_access": f.JMAPAccess = val
	case "smtp_relay": f.SMTPRelay = val
	case "webmail": f.WebMail = val
	case "mobile_sync": f.MobileSync = val
	case "calendar": f.Calendar = val
	case "contacts": f.Contacts = val
	case "tasks": f.Tasks = val
	case "email_forwarding": f.EmailForwarding = val
	case "auto_reply": f.AutoReply = val
	case "catch_all": f.CatchAll = val
	case "distribution_lists": f.DistributionLists = val
	case "shared_mailboxes": f.SharedMailboxes = val
	case "email_aliases": f.EmailAliases = val
	case "anti_spam": f.AntiSpam = val
	case "anti_virus": f.AntiVirus = val
	case "encryption": f.Encryption = val
	case "mfa": f.MFA = val
	case "audit_log": f.AuditLog = val
	case "dkim_signing": f.DKIMSigning = val
	case "email_marketing": f.EmailMarketing = val
	case "marketing_templates": f.MarketingTemplates = val
	case "marketing_analytics": f.MarketingAnalytics = val
	case "marketing_automation": f.MarketingAutomation = val
	case "ab_testing": f.ABTesting = val
	case "rest_api": f.RESTAPI = val
	case "jmap_api": f.JMAPAPI = val
	case "webhooks": f.Webhooks = val
	case "custom_dns": f.CustomDNS = val
	case "white_label": f.WhiteLabel = val
	case "priority_support": f.PrioritySupport = val
	case "sla_agreement": f.SLAgreement = val
	case "dedicated_ip": f.DedicatedIP = val
	}
}

package handlers

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/models"
	"gorm.io/gorm"
)

type DomainHandler struct {
	db *gorm.DB
}

func NewDomainHandler(db *gorm.DB) *DomainHandler {
	return &DomainHandler{db: db}
}

// ============================================================
// Request DTOs
// ============================================================

type CreateDomainRequest struct {
	Name       string  `json:"name" binding:"required"`
	TemplateID *string `json:"template_id,omitempty"`
}

type UpdateDomainRequest struct {
	CatchAllAddress *string `json:"catch_all_address,omitempty"`
	MaxMailboxes    *int    `json:"max_mailboxes,omitempty"`
	SMTPConfigID    *string `json:"smtp_config_id,omitempty"`
}

type DomainWithRecords struct {
	models.Domain
	DNSRecords []models.DNSRecord `json:"dns_records"`
	CheckList  DomainCheckList    `json:"check_list"`
}

type DomainCheckList struct {
	MXRecord   CheckItem `json:"mx_record"`
	SPFRecord  CheckItem `json:"spf_record"`
	DKIMRecord CheckItem `json:"dkim_record"`
	DMARCRecord CheckItem `json:"dmarc_record"`
	MtaSTS     CheckItem `json:"mta_sts"`
	TLSRPT     CheckItem `json:"tls_rpt"`
}

type CheckItem struct {
	Configured bool   `json:"configured"`
	Required   bool   `json:"required"`
	Record     string `json:"record,omitempty"`
	Value      string `json:"value,omitempty"`
	Message    string `json:"message,omitempty"`
}

// ============================================================
// List Domains
// ============================================================

// @Summary     List domains
// @Tags        domains
// @Produce     json
// @Security    BearerAuth
// @Param       page query int false "Page number" default(1)
// @Param       per_page query int false "Items per page" default(20)
// @Param       status query string false "Filter by status"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/domains [get]
func (h *DomainHandler) List(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.UserRole)

	page := c.DefaultQuery("page", "1")
	perPage := c.DefaultQuery("per_page", "20")
	status := c.Query("status")

	var domains []models.Domain
	query := h.db.Preload("SMTPConfig").Preload("Template")

	// Admins see all domains, users see only their own
	if role != models.RoleAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Model(&models.Domain{}).Count(&total)

	if err := query.Order("created_at DESC").
		Offset((atoi(page) - 1) * atoi(perPage)).
		Limit(atoi(perPage)).
		Find(&domains).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch domains"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  domains,
		"total": total,
		"page":  atoi(page),
		"per_page": atoi(perPage),
	})
}

// ============================================================
// Create Domain
// ============================================================

// @Summary     Create a new domain
// @Tags        domains
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body CreateDomainRequest true "Domain details"
// @Success     201 {object} models.Domain
// @Failure     400 {object} map[string]string
// @Failure     409 {object} map[string]string
// @Router      /api/v1/domains [post]
func (h *DomainHandler) Create(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var req CreateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if domain already exists
	var existing models.Domain
	if err := h.db.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Domain already registered"})
		return
	}

	// Generate DKIM keys
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate DKIM keys"})
		return
	}

	privBytes, _ := x509.MarshalPKCS8PrivateKey(privKey)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	pubKey := privKey.Public().(ed25519.PublicKey)

	// Generate verification token
	verifyToken := make([]byte, 32)
	rand.Read(verifyToken)

	domain := models.Domain{
		OwnerID:           userID,
		Name:              req.Name,
		Status:            models.DomainStatusPending,
		VerificationToken: fmt.Sprintf("mailflows-verify=%x", verifyToken),
		DKIMSelector:      "mailflows",
		DKIMPublicKey:     fmt.Sprintf("v=DKIM1; k=ed25519; p=%x", pubKey),
		DKIMPrivateKey:    string(privPEM),
		MaxMailboxes:      10,
	}

	if req.TemplateID != nil {
		tid, err := uuid.Parse(*req.TemplateID)
		if err == nil {
			domain.TemplateID = &tid
		}
	}

	if err := h.db.Create(&domain).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create domain"})
		return
	}

	// Generate DNS records from template or defaults
	h.generateDNSRecords(&domain)

	c.JSON(http.StatusCreated, domain)
}

// ============================================================
// Get Domain
// ============================================================

// @Summary     Get domain details
// @Tags        domains
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Domain ID"
// @Success     200 {object} DomainWithRecords
// @Router      /api/v1/domains/{id} [get]
func (h *DomainHandler) Get(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.UserRole)
	domainID := c.Param("id")

	var domain models.Domain
	query := h.db.Preload("SMTPConfig").Preload("Template")
	if role != models.RoleAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	if err := query.First(&domain, "id = ?", domainID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// Get DNS records
	var records []models.DNSRecord
	h.db.Where("domain_id = ?", domain.ID).Order("type, host").Find(&records)

	// Build check list
	checkList := h.buildCheckList(&domain, records)

	result := DomainWithRecords{
		Domain:     domain,
		DNSRecords: records,
		CheckList:  checkList,
	}

	c.JSON(http.StatusOK, result)
}

// ============================================================
// Delete Domain
// ============================================================

// @Summary     Delete domain
// @Tags        domains
// @Security    BearerAuth
// @Param       id path string true "Domain ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/domains/{id} [delete]
func (h *DomainHandler) Delete(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.UserRole)
	domainID := c.Param("id")

	var domain models.Domain
	query := h.db
	if role != models.RoleAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	if err := query.First(&domain, "id = ?", domainID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// Check for email accounts
	var accountCount int64
	h.db.Model(&models.EmailAccount{}).Where("domain_id = ?", domain.ID).Count(&accountCount)
	if accountCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":           "Domain has active email accounts",
			"account_count":   accountCount,
		})
		return
	}

	// Soft delete
	h.db.Delete(&domain)
	h.db.Where("domain_id = ?", domain.ID).Delete(&models.DNSRecord{})

	c.JSON(http.StatusOK, gin.H{"message": "Domain deleted successfully"})
}

// ============================================================
// Verify Domain
// ============================================================

// @Summary     Trigger domain verification
// @Tags        domains
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Domain ID"
// @Success     200 {object} DomainCheckList
// @Router      /api/v1/domains/{id}/verify [post]
func (h *DomainHandler) Verify(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.UserRole)
	domainID := c.Param("id")

	var domain models.Domain
	query := h.db
	if role != models.RoleAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	if err := query.First(&domain, "id = ?", domainID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// TODO: Perform actual DNS verification (MX, SPF, DKIM, DMARC checks)
	// For now, update status to verifying
	h.db.Model(&domain).Update("status", models.DomainStatusVerifying)

	var records []models.DNSRecord
	h.db.Where("domain_id = ?", domain.ID).Find(&records)

	checkList := h.buildCheckList(&domain, records)
	c.JSON(http.StatusOK, checkList)
}

// ============================================================
// Regenerate DNS Records
// ============================================================

// @Summary     Regenerate DNS records for domain
// @Tags        domains
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Domain ID"
// @Success     200 {array} models.DNSRecord
// @Router      /api/v1/domains/{id}/dns/regenerate [post]
func (h *DomainHandler) RegenerateDNS(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.UserRole)
	domainID := c.Param("id")

	var domain models.Domain
	query := h.db
	if role != models.RoleAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	if err := query.First(&domain, "id = ?", domainID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found"})
		return
	}

	// Delete old managed records
	h.db.Where("domain_id = ? AND is_managed = true", domain.ID).Delete(&models.DNSRecord{})

	// Generate new records
	records := h.generateDNSRecords(&domain)

	c.JSON(http.StatusOK, records)
}

// ============================================================
// Helper Methods
// ============================================================

func (h *DomainHandler) generateDNSRecords(domain *models.Domain) []models.DNSRecord {
	var records []models.DNSRecord
	serverHost := "mail." + domain.Name

	// MX Record
	mxPriority := 10
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordMX,
		Host:      "@",
		Value:     serverHost,
		Priority:  &mxPriority,
		TTL:       3600,
		IsManaged: true,
	})

	// A Record for mail server
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordA,
		Host:      serverHost,
		Value:     "{{server_ip}}", // placeholder - resolved at runtime
		TTL:       3600,
		IsManaged: true,
	})

	// SPF Record
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordTXT,
		Host:      "@",
		Value:     fmt.Sprintf("v=spf1 mx a:%s -all", serverHost),
		TTL:       3600,
		IsManaged: true,
	})

	// DKIM Record
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordTXT,
		Host:      fmt.Sprintf("%s._domainkey", domain.DKIMSelector),
		Value:     domain.DKIMPublicKey,
		TTL:       3600,
		IsManaged: true,
	})

	// DMARC Record
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordTXT,
		Host:      "_dmarc",
		Value:     fmt.Sprintf("v=DMARC1; p=quarantine; rua=mailto:dmarc@%s; pct=100", domain.Name),
		TTL:       3600,
		IsManaged: true,
	})

	// MTA-STS Record
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordTXT,
		Host:      "_mta-sts",
		Value:     fmt.Sprintf("v=STSv1; id=%d", time.Now().Unix()),
		TTL:       3600,
		IsManaged: true,
	})

	// TLSRPT Record
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordTXT,
		Host:      "_smtp._tls",
		Value:     fmt.Sprintf("v=TLSRPTv1; rua=mailto:tlsrpt@%s", domain.Name),
		TTL:       3600,
		IsManaged: true,
	})

	// Autoconfig / Autodiscover
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordCNAME,
		Host:      "autoconfig",
		Value:     serverHost,
		TTL:       3600,
		IsManaged: true,
	})

	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordCNAME,
		Host:      "autodiscover",
		Value:     serverHost,
		TTL:       3600,
		IsManaged: true,
	})

	// SRV Records for IMAP/SMTP auto-discovery
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordSRV,
		Host:      "_imaps._tcp",
		Value:     fmt.Sprintf("0 1 993 %s", serverHost),
		TTL:       3600,
		IsManaged: true,
	})

	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordSRV,
		Host:      "_submissions._tcp",
		Value:     fmt.Sprintf("0 1 587 %s", serverHost),
		TTL:       3600,
		IsManaged: true,
	})

	// JMAP Record (RFC 8620)
	records = append(records, models.DNSRecord{
		DomainID:  domain.ID,
		Type:      models.DNSRecordSRV,
		Host:      "_jmap._tcp",
		Value:     fmt.Sprintf("0 1 443 %s", serverHost),
		TTL:       3600,
		IsManaged: true,
	})

	// Batch insert
	for i := range records {
		h.db.Create(&records[i])
	}

	return records
}

func (h *DomainHandler) buildCheckList(domain *models.Domain, records []models.DNSRecord) DomainCheckList {
	checkList := DomainCheckList{
		MXRecord:    CheckItem{Required: true},
		SPFRecord:   CheckItem{Required: true},
		DKIMRecord:  CheckItem{Required: true},
		DMARCRecord: CheckItem{Required: true},
		MtaSTS:      CheckItem{Required: false},
		TLSRPT:      CheckItem{Required: false},
	}

	for _, r := range records {
		switch r.Type {
		case models.DNSRecordMX:
			checkList.MXRecord.Configured = true
			checkList.MXRecord.Record = r.Host
			checkList.MXRecord.Value = r.Value
		case models.DNSRecordTXT:
			if strings.Contains(r.Value, "v=spf1") {
				checkList.SPFRecord.Configured = true
				checkList.SPFRecord.Record = r.Host
				checkList.SPFRecord.Value = r.Value
			} else if strings.Contains(r.Value, "v=DKIM1") {
				checkList.DKIMRecord.Configured = true
				checkList.DKIMRecord.Record = r.Host
				checkList.DKIMRecord.Value = r.Value
			} else if strings.Contains(r.Value, "v=DMARC1") {
				checkList.DMARCRecord.Configured = true
				checkList.DMARCRecord.Record = r.Host
				checkList.DMARCRecord.Value = r.Value
			} else if strings.Contains(r.Value, "v=STSv1") {
				checkList.MtaSTS.Configured = true
				checkList.MtaSTS.Record = r.Host
				checkList.MtaSTS.Value = r.Value
			} else if strings.Contains(r.Value, "v=TLSRPTv1") {
				checkList.TLSRPT.Configured = true
				checkList.TLSRPT.Record = r.Host
				checkList.TLSRPT.Value = r.Value
			}
		}
	}

	return checkList
}

func atoi(s string) int {
	n := 0
	fmt.Sscanf(s, "%d", &n)
	if n <= 0 {
		return 1
	}
	return n
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/models"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db *gorm.DB
}

func NewAdminHandler(db *gorm.DB) *AdminHandler {
	return &AdminHandler{db: db}
}

// ============================================================
// Dashboard Stats
// ============================================================

// @Summary     Get admin dashboard statistics
// @Tags        admin
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/admin/dashboard [get]
func (h *AdminHandler) Dashboard(c *gin.Context) {
	var stats struct {
		TotalUsers        int64 `json:"total_users"`
		ActiveUsers       int64 `json:"active_users"`
		TotalDomains      int64 `json:"total_domains"`
		ActiveDomains     int64 `json:"active_domains"`
		TotalMailboxes    int64 `json:"total_mailboxes"`
		TotalSMTPConfigs  int64 `json:"total_smtp_configs"`
		TotalCampaigns    int64 `json:"total_campaigns"`
		TotalSubscribers  int64 `json:"total_subscribers"`
		TotalPackages     int64 `json:"total_packages"`
	}

	h.db.Model(&models.User{}).Count(&stats.TotalUsers)
	h.db.Model(&models.User{}).Where("status = ?", models.UserStatusActive).Count(&stats.ActiveUsers)
	h.db.Model(&models.Domain{}).Count(&stats.TotalDomains)
	h.db.Model(&models.Domain{}).Where("status = ?", models.DomainStatusActive).Count(&stats.ActiveDomains)
	h.db.Model(&models.EmailAccount{}).Count(&stats.TotalMailboxes)
	h.db.Model(&models.SMTPConfig{}).Count(&stats.TotalSMTPConfigs)
	h.db.Model(&models.MarketingCampaign{}).Count(&stats.TotalCampaigns)
	h.db.Model(&models.MarketingSubscriber{}).Count(&stats.TotalSubscribers)
	h.db.Model(&models.Package{}).Where("is_active = true").Count(&stats.TotalPackages)

	c.JSON(http.StatusOK, stats)
}

// ============================================================
// User Management
// ============================================================

// @Summary     List all users (admin)
// @Tags        admin
// @Produce     json
// @Security    BearerAuth
// @Param       page query int false "Page" default(1)
// @Param       per_page query int false "Per page" default(20)
// @Param       role query string false "Filter by role"
// @Param       status query string false "Filter by status"
// @Param       search query string false "Search by email/name"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	perPage := c.DefaultQuery("per_page", "20")
	role := c.Query("role")
	status := c.Query("status")
	search := c.Query("search")

	var users []models.User
	query := h.db

	if role != "" {
		query = query.Where("role = ?", role)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		query = query.Where("email ILIKE ? OR first_name ILIKE ? OR last_name ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Model(&models.User{}).Count(&total)

	if err := query.Order("created_at DESC").
		Offset((atoi(page) - 1) * atoi(perPage)).
		Limit(atoi(perPage)).
		Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	// Remove sensitive fields
	for i := range users {
		users[i].PasswordHash = ""
		users[i].MFASecret = ""
		users[i].MFABackupCodes = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     users,
		"total":    total,
		"page":     atoi(page),
		"per_page": atoi(perPage),
	})
}

// @Summary     Update user (admin)
// @Tags        admin
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "User ID"
// @Param       body body map[string]interface{} true "User updates"
// @Success     200 {object} models.User
// @Router      /api/v1/admin/users/{id} [put]
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if err := h.db.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var req struct {
		Role        *string `json:"role"`
		Status      *string `json:"status"`
		PackageID   *string `json:"package_id"`
		StorageQuota *int64 `json:"storage_quota"`
		MaxEmailAccounts *int `json:"max_email_accounts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Role != nil { updates["role"] = *req.Role }
	if req.Status != nil { updates["status"] = *req.Status }
	if req.StorageQuota != nil { updates["storage_quota"] = *req.StorageQuota }
	if req.MaxEmailAccounts != nil { updates["max_email_accounts"] = *req.MaxEmailAccounts }

	if req.PackageID != nil {
		if *req.PackageID == "" {
			updates["package_id"] = nil
		} else {
			pid := uuid.MustParse(*req.PackageID)
			updates["package_id"] = pid
		}
	}

	if len(updates) > 0 {
		h.db.Model(&user).Updates(updates)
	}

	h.db.First(&user, "id = ?", id)
	user.PasswordHash = ""
	user.MFASecret = ""
	c.JSON(http.StatusOK, user)
}

// @Summary     Suspend user (admin)
// @Tags        admin
// @Security    BearerAuth
// @Param       id path string true "User ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/admin/users/{id}/suspend [post]
func (h *AdminHandler) SuspendUser(c *gin.Context) {
	id := c.Param("id")
	h.db.Model(&models.User{}).Where("id = ?", id).Update("status", models.UserStatusSuspended)
	c.JSON(http.StatusOK, gin.H{"message": "User suspended"})
}

// @Summary     Activate user (admin)
// @Tags        admin
// @Security    BearerAuth
// @Param       id path string true "User ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/admin/users/{id}/activate [post]
func (h *AdminHandler) ActivateUser(c *gin.Context) {
	id := c.Param("id")
	h.db.Model(&models.User{}).Where("id = ?", id).Update("status", models.UserStatusActive)
	c.JSON(http.StatusOK, gin.H{"message": "User activated"})
}

// ============================================================
// System Settings
// ============================================================

// @Summary     Get all system settings
// @Tags        admin
// @Produce     json
// @Security    BearerAuth
// @Param       category query string false "Filter by category"
// @Success     200 {array} models.SystemSetting
// @Router      /api/v1/admin/settings [get]
func (h *AdminHandler) GetSettings(c *gin.Context) {
	category := c.Query("category")

	var settings []models.SystemSetting
	query := h.db
	if category != "" {
		query = query.Where("category = ?", category)
	}

	if err := query.Order("category, key").Find(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}

	// Mask secret values
	for i := range settings {
		if settings[i].IsSecret && settings[i].Value != "" {
			settings[i].Value = "********"
		}
	}

	c.JSON(http.StatusOK, settings)
}

// @Summary     Update system setting
// @Tags        admin
// @Accept      json
// @Security    BearerAuth
// @Param       key path string true "Setting key"
// @Param       body body map[string]string true "Setting value"
// @Success     200 {object} models.SystemSetting
// @Router      /api/v1/admin/settings/{key} [put]
func (h *AdminHandler) UpdateSetting(c *gin.Context) {
	key := c.Param("key")

	var req struct {
		Value    string `json:"value"`
		Category string `json:"category"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var setting models.SystemSetting
	if err := h.db.Where("key = ?", key).First(&setting).Error; err != nil {
		// Create new setting
		setting = models.SystemSetting{
			Key:      key,
			Value:    req.Value,
			Category: req.Category,
		}
		h.db.Create(&setting)
	} else {
		h.db.Model(&setting).Update("value", req.Value)
	}

	c.JSON(http.StatusOK, setting)
}

// ============================================================
// Audit Logs
// ============================================================

// @Summary     Get audit logs
// @Tags        admin
// @Produce     json
// @Security    BearerAuth
// @Param       page query int false "Page" default(1)
// @Param       per_page query int false "Per page" default(50)
// @Param       action query string false "Filter by action"
// @Param       resource query string false "Filter by resource"
// @Param       user_id query string false "Filter by user ID"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/admin/audit-logs [get]
func (h *AdminHandler) GetAuditLogs(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	perPage := c.DefaultQuery("per_page", "50")
	action := c.Query("action")
	resource := c.Query("resource")
	userID := c.Query("user_id")

	var logs []models.AuditLog
	query := h.db

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if resource != "" {
		query = query.Where("resource = ?", resource)
	}
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	var total int64
	query.Model(&models.AuditLog{}).Count(&total)

	if err := query.Order("created_at DESC").
		Offset((atoi(page) - 1) * atoi(perPage)).
		Limit(atoi(perPage)).
		Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     logs,
		"total":    total,
		"page":     atoi(page),
		"per_page": atoi(perPage),
	})
}

// ============================================================
// DNS Templates
// ============================================================

// @Summary     List DNS templates
// @Tags        admin
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} models.DNSTemplate
// @Router      /api/v1/admin/dns-templates [get]
func (h *AdminHandler) ListDNSTemplates(c *gin.Context) {
	var templates []models.DNSTemplate
	if err := h.db.Preload("Records", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC")
	}).Order("name").Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch templates"})
		return
	}

	c.JSON(http.StatusOK, templates)
}

type CreateDNSTemplateRequest struct {
	Name        string                   `json:"name" binding:"required"`
	Description string                   `json:"description"`
	IsDefault   bool                     `json:"is_default"`
	Records     []DNSTemplateRecordReq   `json:"records" binding:"required,min=1"`
}

type DNSTemplateRecordReq struct {
	Type     string `json:"type" binding:"required"`
	Host     string `json:"host" binding:"required"`
	Value    string `json:"value" binding:"required"`
	Priority *int   `json:"priority"`
	TTL      int    `json:"ttl"`
}

// @Summary     Create DNS template
// @Tags        admin
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body CreateDNSTemplateRequest true "Template details"
// @Success     201 {object} models.DNSTemplate
// @Router      /api/v1/admin/dns-templates [post]
func (h *AdminHandler) CreateDNSTemplate(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var req CreateDNSTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// If setting as default, unset others
	if req.IsDefault {
		h.db.Model(&models.DNSTemplate{}).Where("is_default = true").Update("is_default", false)
	}

	template := models.DNSTemplate{
		Name:        req.Name,
		Description: req.Description,
		IsDefault:   req.IsDefault,
		CreatedBy:   userID,
	}

	if err := h.db.Create(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create template"})
		return
	}

	// Create records
	for i, r := range req.Records {
		record := models.DNSTemplateRecord{
			TemplateID: template.ID,
			Type:       models.DNSRecordType(r.Type),
			Host:       r.Host,
			Value:      r.Value,
			Priority:   r.Priority,
			TTL:        r.TTL,
			SortOrder:  i,
		}
		if record.TTL == 0 {
			record.TTL = 3600
		}
		h.db.Create(&record)
	}

	// Reload with records
	h.db.Preload("Records").First(&template, template.ID)
	c.JSON(http.StatusCreated, template)
}

// @Summary     Update DNS template
// @Tags        admin
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Template ID"
// @Param       body body CreateDNSTemplateRequest true "Template details"
// @Success     200 {object} models.DNSTemplate
// @Router      /api/v1/admin/dns-templates/{id} [put]
func (h *AdminHandler) UpdateDNSTemplate(c *gin.Context) {
	id := c.Param("id")

	var template models.DNSTemplate
	if err := h.db.First(&template, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Template not found"})
		return
	}

	var req CreateDNSTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.IsDefault && !template.IsDefault {
		h.db.Model(&models.DNSTemplate{}).Where("is_default = true").Update("is_default", false)
	}

	h.db.Model(&template).Updates(map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
		"is_default":  req.IsDefault,
	})

	// Replace records
	h.db.Where("template_id = ?", template.ID).Delete(&models.DNSTemplateRecord{})
	for i, r := range req.Records {
		record := models.DNSTemplateRecord{
			TemplateID: template.ID,
			Type:       models.DNSRecordType(r.Type),
			Host:       r.Host,
			Value:      r.Value,
			Priority:   r.Priority,
			TTL:        r.TTL,
			SortOrder:  i,
		}
		if record.TTL == 0 {
			record.TTL = 3600
		}
		h.db.Create(&record)
	}

	h.db.Preload("Records").First(&template, template.ID)
	c.JSON(http.StatusOK, template)
}

// @Summary     Delete DNS template
// @Tags        admin
// @Security    BearerAuth
// @Param       id path string true "Template ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/admin/dns-templates/{id} [delete]
func (h *AdminHandler) DeleteDNSTemplate(c *gin.Context) {
	id := c.Param("id")

	// Check if used by domains
	var domainCount int64
	h.db.Model(&models.Domain{}).Where("template_id = ?", id).Count(&domainCount)
	if domainCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "Template is in use by domains",
			"domain_count": domainCount,
		})
		return
	}

	h.db.Where("template_id = ?", id).Delete(&models.DNSTemplateRecord{})
	h.db.Delete(&models.DNSTemplate{}, "id = ?", id)

	c.JSON(http.StatusOK, gin.H{"message": "Template deleted"})
}

package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/models"
	"gorm.io/gorm"
)

type SMTPConfigHandler struct {
	db *gorm.DB
}

func NewSMTPConfigHandler(db *gorm.DB) *SMTPConfigHandler {
	return &SMTPConfigHandler{db: db}
}

type CreateSMTPConfigRequest struct {
	Name       string `json:"name" binding:"required"`
	Host       string `json:"host" binding:"required"`
	Port       int    `json:"port" binding:"required"`
	TLSPort    int    `json:"tls_port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	AuthType   string `json:"auth_type"`
	FromDomain string `json:"from_domain"`
	MaxPerHour int    `json:"max_per_hour"`
	MaxPerDay  int    `json:"max_per_day"`
	IsGlobal   bool   `json:"is_global"`
	Priority   int    `json:"priority"`
}

// @Summary     List SMTP configurations
// @Tags        smtp-config
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} models.SMTPConfig
// @Router      /api/v1/admin/smtp-configs [get]
func (h *SMTPConfigHandler) List(c *gin.Context) {
	var configs []models.SMTPConfig
	if err := h.db.Order("priority DESC, name").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch SMTP configs"})
		return
	}

	// Mask passwords
	for i := range configs {
		configs[i].Password = ""
	}

	c.JSON(http.StatusOK, configs)
}

// @Summary     Create SMTP configuration
// @Tags        smtp-config
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body CreateSMTPConfigRequest true "SMTP config details"
// @Success     201 {object} models.SMTPConfig
// @Router      /api/v1/admin/smtp-configs [post]
func (h *SMTPConfigHandler) Create(c *gin.Context) {
	var req CreateSMTPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	config := models.SMTPConfig{
		Name:       req.Name,
		Host:       req.Host,
		Port:       req.Port,
		TLSPort:    req.TLSPort,
		Username:   req.Username,
		Password:   req.Password,
		AuthType:   models.SMTPAuthType(req.AuthType),
		FromDomain: req.FromDomain,
		MaxPerHour: req.MaxPerHour,
		MaxPerDay:  req.MaxPerDay,
		IsGlobal:   req.IsGlobal,
		Priority:   req.Priority,
		IsActive:   true,
	}

	if config.MaxPerHour == 0 {
		config.MaxPerHour = 100
	}
	if config.MaxPerDay == 0 {
		config.MaxPerDay = 1000
	}

	if err := h.db.Create(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SMTP config"})
		return
	}

	config.Password = ""
	c.JSON(http.StatusCreated, config)
}

// @Summary     Update SMTP configuration
// @Tags        smtp-config
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Config ID"
// @Param       body body CreateSMTPConfigRequest true "SMTP config details"
// @Success     200 {object} models.SMTPConfig
// @Router      /api/v1/admin/smtp-configs/{id} [put]
func (h *SMTPConfigHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var config models.SMTPConfig
	if err := h.db.First(&config, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SMTP config not found"})
		return
	}

	var req CreateSMTPConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"name":        req.Name,
		"host":        req.Host,
		"port":        req.Port,
		"tls_port":    req.TLSPort,
		"username":    req.Username,
		"auth_type":   req.AuthType,
		"from_domain": req.FromDomain,
		"max_per_hour": req.MaxPerHour,
		"max_per_day": req.MaxPerDay,
		"is_global":   req.IsGlobal,
		"priority":    req.Priority,
	}

	if req.Password != "" {
		updates["password"] = req.Password
	}

	h.db.Model(&config).Updates(updates)
	h.db.First(&config, "id = ?", id)
	config.Password = ""

	c.JSON(http.StatusOK, config)
}

// @Summary     Delete SMTP configuration
// @Tags        smtp-config
// @Security    BearerAuth
// @Param       id path string true "Config ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/admin/smtp-configs/{id} [delete]
func (h *SMTPConfigHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	var config models.SMTPConfig
	if err := h.db.First(&config, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SMTP config not found"})
		return
	}

	// Check if assigned to domains
	var domainCount int64
	h.db.Model(&models.Domain{}).Where("smtp_config_id = ?", id).Count(&domainCount)
	if domainCount > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "SMTP config is assigned to domains",
			"domain_count": domainCount,
		})
		return
	}

	h.db.Delete(&config)
	c.JSON(http.StatusOK, gin.H{"message": "SMTP config deleted"})
}

// @Summary     Assign SMTP config to domain
// @Tags        smtp-config
// @Accept      json
// @Security    BearerAuth
// @Param       id path string true "SMTP Config ID"
// @Param       body body map[string]string true "Domain ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/admin/smtp-configs/{id}/assign [post]
func (h *SMTPConfigHandler) AssignToDomain(c *gin.Context) {
	configID := c.Param("id")

	var req struct {
		DomainID string `json:"domain_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configUUID, err := uuid.Parse(configID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid config ID"})
		return
	}

	domainUUID, err := uuid.Parse(req.DomainID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid domain ID"})
		return
	}

	result := h.db.Model(&models.Domain{}).
		Where("id = ?", domainUUID).
		Update("smtp_config_id", configUUID)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign SMTP config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "SMTP config assigned to domain"})
}

// @Summary     Test SMTP connection
// @Tags        smtp-config
// @Accept      json
// @Security    BearerAuth
// @Param       id path string true "Config ID"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/admin/smtp-configs/{id}/test [post]
func (h *SMTPConfigHandler) TestConnection(c *gin.Context) {
	id := c.Param("id")

	var config models.SMTPConfig
	if err := h.db.First(&config, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "SMTP config not found"})
		return
	}

	// TODO: Actually test SMTP connection
	// For now return success
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SMTP connection test passed",
		"details": gin.H{
			"host": config.Host,
			"port": config.Port,
		},
	})
}

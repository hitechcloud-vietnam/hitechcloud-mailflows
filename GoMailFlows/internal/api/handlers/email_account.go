package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type EmailAccountHandler struct {
	db *gorm.DB
}

func NewEmailAccountHandler(db *gorm.DB) *EmailAccountHandler {
	return &EmailAccountHandler{db: db}
}

type CreateEmailAccountRequest struct {
	DomainID     string `json:"domain_id" binding:"required"`
	LocalPart    string `json:"local_part" binding:"required"` // part before @
	Password     string `json:"password" binding:"required,min=8"`
	DisplayName  string `json:"display_name"`
	StorageQuota int64  `json:"storage_quota"`
}

type UpdateEmailAccountRequest struct {
	DisplayName  *string `json:"display_name,omitempty"`
	IsEnabled    *bool   `json:"is_enabled,omitempty"`
	StorageQuota *int64  `json:"storage_quota,omitempty"`
	MaxSendPerDay *int   `json:"max_send_per_day,omitempty"`
	ForwardTo    *string `json:"forward_to,omitempty"`
	AutoReply    *bool   `json:"auto_reply,omitempty"`
	AutoReplyText *string `json:"auto_reply_text,omitempty"`
	Password     *string `json:"password,omitempty"`
}

// @Summary     List email accounts
// @Tags        email-accounts
// @Produce     json
// @Security    BearerAuth
// @Param       domain_id query string false "Filter by domain ID"
// @Param       page query int false "Page" default(1)
// @Param       per_page query int false "Per page" default(20)
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/email-accounts [get]
func (h *EmailAccountHandler) List(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.UserRole)
	domainID := c.Query("domain_id")
	page := c.DefaultQuery("page", "1")
	perPage := c.DefaultQuery("per_page", "20")

	var accounts []models.EmailAccount
	query := h.db.Preload("Domain")

	if role != models.RoleAdmin {
		query = query.Where("user_id = ?", userID)
	}

	if domainID != "" {
		query = query.Where("domain_id = ?", domainID)
	}

	var total int64
	query.Model(&models.EmailAccount{}).Count(&total)

	if err := query.Order("address ASC").
		Offset((atoi(page) - 1) * atoi(perPage)).
		Limit(atoi(perPage)).
		Find(&accounts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch accounts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     accounts,
		"total":    total,
		"page":     atoi(page),
		"per_page": atoi(perPage),
	})
}

// @Summary     Create email account
// @Tags        email-accounts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body CreateEmailAccountRequest true "Account details"
// @Success     201 {object} models.EmailAccount
// @Router      /api/v1/email-accounts [post]
func (h *EmailAccountHandler) Create(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var req CreateEmailAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify domain exists and user owns it
	var domain models.Domain
	if err := h.db.Where("id = ? AND owner_id = ?", req.DomainID, userID).First(&domain).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Domain not found or not owned by you"})
		return
	}

	if domain.Status != models.DomainStatusActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Domain is not active"})
		return
	}

	// Check mailbox quota
	if domain.MailboxCount >= domain.MaxMailboxes {
		c.JSON(http.StatusForbidden, gin.H{"error": "Mailbox limit reached for this domain"})
		return
	}

	// Validate local part
	localPart := strings.ToLower(strings.TrimSpace(req.LocalPart))
	if !isValidLocalPart(localPart) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid local part"})
		return
	}

	address := fmt.Sprintf("%s@%s", localPart, domain.Name)

	// Check if address already exists
	var existing models.EmailAccount
	if err := h.db.Where("address = ?", address).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email address already exists"})
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	quota := req.StorageQuota
	if quota == 0 {
		quota = 1073741824 // 1GB default
	}

	account := models.EmailAccount{
		UserID:       userID,
		DomainID:     domain.ID,
		Address:      address,
		PasswordHash: string(hash),
		DisplayName:  req.DisplayName,
		IsEnabled:    true,
		StorageQuota: quota,
		MaxSendPerDay: 500,
	}

	if err := h.db.Create(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create email account"})
		return
	}

	// Increment domain mailbox count
	h.db.Model(&domain).Update("mailbox_count", gorm.Expr("mailbox_count + 1"))

	// Increment user email account count
	h.db.Model(&models.User{}).Where("id = ?", userID).
		Update("email_account_count", gorm.Expr("email_account_count + 1"))

	c.JSON(http.StatusCreated, account)
}

// @Summary     Update email account
// @Tags        email-accounts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Account ID"
// @Param       body body UpdateEmailAccountRequest true "Update details"
// @Success     200 {object} models.EmailAccount
// @Router      /api/v1/email-accounts/{id} [put]
func (h *EmailAccountHandler) Update(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.UserRole)
	accountID := c.Param("id")

	var account models.EmailAccount
	query := h.db
	if role != models.RoleAdmin {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.First(&account, "id = ?", accountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	var req UpdateEmailAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.DisplayName != nil { updates["display_name"] = *req.DisplayName }
	if req.IsEnabled != nil { updates["is_enabled"] = *req.IsEnabled }
	if req.StorageQuota != nil { updates["storage_quota"] = *req.StorageQuota }
	if req.MaxSendPerDay != nil { updates["max_send_per_day"] = *req.MaxSendPerDay }
	if req.ForwardTo != nil { updates["forward_to"] = *req.ForwardTo }
	if req.AutoReply != nil { updates["auto_reply"] = *req.AutoReply }
	if req.AutoReplyText != nil { updates["auto_reply_text"] = *req.AutoReplyText }

	if req.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		updates["password_hash"] = string(hash)
	}

	if len(updates) > 0 {
		h.db.Model(&account).Updates(updates)
	}

	h.db.First(&account, "id = ?", accountID)
	c.JSON(http.StatusOK, account)
}

// @Summary     Delete email account
// @Tags        email-accounts
// @Security    BearerAuth
// @Param       id path string true "Account ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/email-accounts/{id} [delete]
func (h *EmailAccountHandler) Delete(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	role := c.MustGet("user_role").(models.UserRole)
	accountID := c.Param("id")

	var account models.EmailAccount
	query := h.db
	if role != models.RoleAdmin {
		query = query.Where("user_id = ?", userID)
	}

	if err := query.First(&account, "id = ?", accountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	h.db.Delete(&account)

	// Decrement domain mailbox count
	h.db.Model(&models.Domain{}).Where("id = ?", account.DomainID).
		Update("mailbox_count", gorm.Expr("GREATEST(mailbox_count - 1, 0)"))

	// Decrement user email account count
	h.db.Model(&models.User{}).Where("id = ?", account.UserID).
		Update("email_account_count", gorm.Expr("GREATEST(email_account_count - 1, 0)"))

	c.JSON(http.StatusOK, gin.H{"message": "Email account deleted"})
}

func isValidLocalPart(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' || c == '+') {
			return false
		}
	}
	return true
}

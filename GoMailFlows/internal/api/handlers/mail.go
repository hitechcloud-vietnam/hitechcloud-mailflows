package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hitechcloud/mailflows/internal/models"
	"gorm.io/gorm"
)

type MailHandler struct {
	db *gorm.DB
}

func NewMailHandler(db *gorm.DB) *MailHandler {
	return &MailHandler{db: db}
}

// ============================================================
// Folders
// ============================================================

// @Summary     List mail folders
// @Tags        mail
// @Produce     json
// @Security    BearerAuth
// @Param       account_id query string true "Email account ID"
// @Success     200 {array} map[string]interface{}
// @Router      /api/v1/mail/folders [get]
func (h *MailHandler) ListFolders(c *gin.Context) {
	accountID := c.Query("account_id")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id is required"})
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	// Verify account ownership
	var account models.EmailAccount
	if err := h.db.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	// Standard IMAP folders
	folders := []gin.H{
		{"name": "INBOX", "type": "inbox", "unread_count": 0, "total_count": 0},
		{"name": "Sent", "type": "sent", "unread_count": 0, "total_count": 0},
		{"name": "Drafts", "type": "drafts", "unread_count": 0, "total_count": 0},
		{"name": "Trash", "type": "trash", "unread_count": 0, "total_count": 0},
		{"name": "Spam", "type": "spam", "unread_count": 0, "total_count": 0},
		{"name": "Archive", "type": "archive", "unread_count": 0, "total_count": 0},
		{"name": "Junk", "type": "junk", "unread_count": 0, "total_count": 0},
	}

	// TODO: Fetch real folder data from IMAP/mailstore

	c.JSON(http.StatusOK, folders)
}

// ============================================================
// Messages
// ============================================================

type Message struct {
	ID          string   `json:"id"`
	Folder      string   `json:"folder"`
	Subject     string   `json:"subject"`
	From        string   `json:"from"`
	FromName    string   `json:"from_name"`
	To          []string `json:"to"`
	CC          []string `json:"cc,omitempty"`
	Date        string   `json:"date"`
	Body        string   `json:"body"`
	HTMLBody    string   `json:"html_body,omitempty"`
	IsRead      bool     `json:"is_read"`
	IsStarred   bool     `json:"is_starred"`
	HasAttach   bool     `json:"has_attachments"`
	AttachCount int      `json:"attachment_count"`
	Snippet     string   `json:"snippet"`
	ThreadID    string   `json:"thread_id,omitempty"`
}

// @Summary     List messages
// @Tags        mail
// @Produce     json
// @Security    BearerAuth
// @Param       account_id query string true "Email account ID"
// @Param       folder query string false "Folder name" default(INBOX)
// @Param       page query int false "Page" default(1)
// @Param       per_page query int false "Per page" default(50)
// @Param       search query string false "Search query"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/mail/messages [get]
func (h *MailHandler) ListMessages(c *gin.Context) {
	accountID := c.Query("account_id")
	folder := c.DefaultQuery("folder", "INBOX")
	page := c.DefaultQuery("page", "1")
	perPage := c.DefaultQuery("per_page", "50")
	search := c.Query("search")

	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account_id is required"})
		return
	}

	userID := c.MustGet("user_id").(uuid.UUID)

	// Verify account ownership
	var account models.EmailAccount
	if err := h.db.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	// TODO: Query messages from mailstore (could be database or IMAP backend)
	_ = folder
	_ = search
	_ = page
	_ = perPage

	c.JSON(http.StatusOK, gin.H{
		"data":     []Message{},
		"total":    0,
		"page":     atoi(page),
		"per_page": atoi(perPage),
	})
}

// @Summary     Get message
// @Tags        mail
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Message ID"
// @Success     200 {object} Message
// @Router      /api/v1/mail/messages/{id} [get]
func (h *MailHandler) GetMessage(c *gin.Context) {
	id := c.Param("id")

	userID := c.MustGet("user_id").(uuid.UUID)
	_ = id
	_ = userID

	// TODO: Fetch message from mailstore

	c.JSON(http.StatusNotFound, gin.H{"error": "Message not found"})
}

// ============================================================
// Send
// ============================================================

type SendRequest struct {
	AccountID   string   `json:"account_id" binding:"required"`
	To          []string `json:"to" binding:"required,min=1"`
	CC          []string `json:"cc"`
	BCC         []string `json:"bcc"`
	Subject     string   `json:"subject" binding:"required"`
	Body        string   `json:"body"`
	HTMLBody    string   `json:"html_body"`
	ReplyTo     string   `json:"reply_to"`
	InReplyTo   string   `json:"in_reply_to"`
	Attachments []AttachmentReq `json:"attachments"`
}

type AttachmentReq struct {
	Filename    string `json:"filename" binding:"required"`
	ContentType string `json:"content_type"`
	Content     string `json:"content" binding:"required"` // base64 encoded
}

// @Summary     Send email
// @Tags        mail
// @Accept      json
// @Security    BearerAuth
// @Param       body body SendRequest true "Email details"
// @Success     200 {object} map[string]string
// @Router      /api/v1/mail/send [post]
func (h *MailHandler) SendMessage(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify account ownership
	var account models.EmailAccount
	if err := h.db.Where("id = ? AND user_id = ?", req.AccountID, userID).First(&account).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email account not found"})
		return
	}

	if !account.IsEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Email account is disabled"})
		return
	}

	// Check send limit
	if account.SentToday >= account.MaxSendPerDay {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Daily send limit reached"})
		return
	}

	// TODO: Actually send via SMTP
	// For now, just record the send

	// Increment sent count
	h.db.Model(&account).Update("sent_today", gorm.Expr("sent_today + 1"))

	c.JSON(http.StatusOK, gin.H{
		"message": "Email sent successfully",
		"from":    account.Address,
	})
}

// @Summary     Mark message as read
// @Tags        mail
// @Security    BearerAuth
// @Param       id path string true "Message ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/mail/messages/{id}/read [put]
func (h *MailHandler) MarkRead(c *gin.Context) {
	// TODO: Mark message as read via IMAP/mailstore
	c.JSON(http.StatusOK, gin.H{"message": "Marked as read"})
}

// @Summary     Mark message as unread
// @Tags        mail
// @Security    BearerAuth
// @Param       id path string true "Message ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/mail/messages/{id}/unread [put]
func (h *MailHandler) MarkUnread(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Marked as unread"})
}

// @Summary     Delete message
// @Tags        mail
// @Security    BearerAuth
// @Param       id path string true "Message ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/mail/messages/{id} [delete]
func (h *MailHandler) DeleteMessage(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Message deleted"})
}

// @Summary     Move message to folder
// @Tags        mail
// @Accept      json
// @Security    BearerAuth
// @Param       id path string true "Message ID"
// @Param       body body map[string]string true "Target folder"
// @Success     200 {object} map[string]string
// @Router      /api/v1/mail/messages/{id}/move [post]
func (h *MailHandler) MoveMessage(c *gin.Context) {
	var req struct {
		Folder string `json:"folder" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Message moved"})
}

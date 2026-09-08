package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/models"
	"gorm.io/gorm"
)

type MarketingHandler struct {
	db *gorm.DB
}

func NewMarketingHandler(db *gorm.DB) *MarketingHandler {
	return &MarketingHandler{db: db}
}

// ============================================================
// Campaigns
// ============================================================

type CreateCampaignRequest struct {
	Name        string   `json:"name" binding:"required"`
	Subject     string   `json:"subject" binding:"required"`
	FromName    string   `json:"from_name"`
	FromEmail   string   `json:"from_email" binding:"required,email"`
	ReplyTo     string   `json:"reply_to"`
	HTMLContent string   `json:"html_content"`
	PlainText   string   `json:"plain_text"`
	Tags        []string `json:"tags"`
}

// @Summary     List marketing campaigns
// @Tags        marketing
// @Produce     json
// @Security    BearerAuth
// @Param       status query string false "Filter by status"
// @Success     200 {array} models.MarketingCampaign
// @Router      /api/v1/marketing/campaigns [get]
func (h *MarketingHandler) ListCampaigns(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	status := c.Query("status")

	var campaigns []models.MarketingCampaign
	query := h.db.Where("user_id = ?", userID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&campaigns).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch campaigns"})
		return
	}

	c.JSON(http.StatusOK, campaigns)
}

// @Summary     Create marketing campaign
// @Tags        marketing
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body CreateCampaignRequest true "Campaign details"
// @Success     201 {object} models.MarketingCampaign
// @Router      /api/v1/marketing/campaigns [post]
func (h *MarketingHandler) CreateCampaign(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var req CreateCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	campaign := models.MarketingCampaign{
		UserID:      userID,
		Name:        req.Name,
		Subject:     req.Subject,
		FromName:    req.FromName,
		FromEmail:   req.FromEmail,
		ReplyTo:     req.ReplyTo,
		HTMLContent: req.HTMLContent,
		PlainText:   req.PlainText,
		Status:      models.CampaignStatusDraft,
		Tags:        req.Tags,
	}

	if err := h.db.Create(&campaign).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create campaign"})
		return
	}

	c.JSON(http.StatusCreated, campaign)
}

// @Summary     Get campaign details
// @Tags        marketing
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Campaign ID"
// @Success     200 {object} models.MarketingCampaign
// @Router      /api/v1/marketing/campaigns/{id} [get]
func (h *MarketingHandler) GetCampaign(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	id := c.Param("id")

	var campaign models.MarketingCampaign
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&campaign).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

// @Summary     Schedule campaign
// @Tags        marketing
// @Accept      json
// @Security    BearerAuth
// @Param       id path string true "Campaign ID"
// @Param       body body map[string]string true "Schedule time"
// @Success     200 {object} map[string]string
// @Router      /api/v1/marketing/campaigns/{id}/schedule [post]
func (h *MarketingHandler) ScheduleCampaign(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	id := c.Param("id")

	var req struct {
		ScheduledAt time.Time `json:"scheduled_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var campaign models.MarketingCampaign
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&campaign).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if campaign.Status != models.CampaignStatusDraft {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campaign is not in draft status"})
		return
	}

	h.db.Model(&campaign).Updates(map[string]interface{}{
		"status":       models.CampaignStatusScheduled,
		"scheduled_at": req.ScheduledAt,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Campaign scheduled", "scheduled_at": req.ScheduledAt})
}

// @Summary     Send campaign now
// @Tags        marketing
// @Security    BearerAuth
// @Param       id path string true "Campaign ID"
// @Success     200 {object} map[string]string
// @Router      /api/v1/marketing/campaigns/{id}/send [post]
func (h *MarketingHandler) SendCampaign(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	id := c.Param("id")

	var campaign models.MarketingCampaign
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&campaign).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	if campaign.Status != models.CampaignStatusDraft && campaign.Status != models.CampaignStatusScheduled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Campaign cannot be sent"})
		return
	}

	// TODO: Queue campaign for sending via worker
	now := time.Now()
	h.db.Model(&campaign).Updates(map[string]interface{}{
		"status":  models.CampaignStatusSending,
		"sent_at": &now,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Campaign queued for sending"})
}

// ============================================================
// Subscriber Lists
// ============================================================

type CreateListRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// @Summary     Create subscriber list
// @Tags        marketing
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body CreateListRequest true "List details"
// @Success     201 {object} models.MarketingList
// @Router      /api/v1/marketing/lists [post]
func (h *MarketingHandler) CreateList(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var req CreateListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	list := models.MarketingList{
		UserID:      userID,
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
	}

	if err := h.db.Create(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create list"})
		return
	}

	c.JSON(http.StatusCreated, list)
}

// @Summary     List subscriber lists
// @Tags        marketing
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} models.MarketingList
// @Router      /api/v1/marketing/lists [get]
func (h *MarketingHandler) Lists(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var lists []models.MarketingList
	if err := h.db.Where("user_id = ?", userID).Order("name").Find(&lists).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch lists"})
		return
	}

	c.JSON(http.StatusOK, lists)
}

// ============================================================
// Subscribers
// ============================================================

type AddSubscriberRequest struct {
	Email     string            `json:"email" binding:"required,email"`
	FirstName string            `json:"first_name"`
	LastName  string            `json:"last_name"`
	MetaData  map[string]string `json:"meta_data"`
}

type BulkAddSubscribersRequest struct {
	ListID      string                `json:"list_id" binding:"required"`
	Subscribers []AddSubscriberRequest `json:"subscribers" binding:"required,min=1"`
}

// @Summary     Add subscriber to list
// @Tags        marketing
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       listId path string true "List ID"
// @Param       body body AddSubscriberRequest true "Subscriber details"
// @Success     201 {object} models.MarketingSubscriber
// @Router      /api/v1/marketing/lists/{listId}/subscribers [post]
func (h *MarketingHandler) AddSubscriber(c *gin.Context) {
	listID := c.Param("listId")

	var req AddSubscriberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if already subscribed
	var existing models.MarketingSubscriber
	if err := h.db.Where("list_id = ? AND email = ?", listID, req.Email).First(&existing).Error; err == nil {
		if existing.Status == models.SubscriberStatusUnsubscribed {
			// Re-subscribe
			h.db.Model(&existing).Updates(map[string]interface{}{
				"status":           models.SubscriberStatusActive,
				"unsubscribed_at":  nil,
			})
			c.JSON(http.StatusOK, existing)
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "Email already subscribed"})
		return
	}

	subscriber := models.MarketingSubscriber{
		ListID:    uuid.MustParse(listID),
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Status:    models.SubscriberStatusActive,
		MetaData:  req.MetaData,
	}

	if err := h.db.Create(&subscriber).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add subscriber"})
		return
	}

	// Update list subscriber count
	h.db.Model(&models.MarketingList{}).Where("id = ?", listID).
		Update("subscriber_count", gorm.Expr("subscriber_count + 1"))

	c.JSON(http.StatusCreated, subscriber)
}

// @Summary     Bulk add subscribers
// @Tags        marketing
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body BulkAddSubscribersRequest true "Bulk subscribers"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/marketing/subscribers/bulk [post]
func (h *MarketingHandler) BulkAddSubscribers(c *gin.Context) {
	var req BulkAddSubscribersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	added := 0
	skipped := 0

	for _, sub := range req.Subscribers {
		var existing models.MarketingSubscriber
		if err := h.db.Where("list_id = ? AND email = ?", req.ListID, sub.Email).First(&existing).Error; err == nil {
			skipped++
			continue
		}

		subscriber := models.MarketingSubscriber{
			ListID:    uuid.MustParse(req.ListID),
			Email:     sub.Email,
			FirstName: sub.FirstName,
			LastName:  sub.LastName,
			Status:    models.SubscriberStatusActive,
			MetaData:  sub.MetaData,
		}

		if err := h.db.Create(&subscriber).Error; err == nil {
			added++
		}
	}

	// Update list count
	h.db.Model(&models.MarketingList{}).Where("id = ?", req.ListID).
		Update("subscriber_count", gorm.Expr("subscriber_count + ?", added))

	c.JSON(http.StatusOK, gin.H{
		"added":   added,
		"skipped": skipped,
		"total":   len(req.Subscribers),
	})
}

// @Summary     Unsubscribe from list
// @Tags        marketing
// @Security    BearerAuth
// @Param       listId path string true "List ID"
// @Param       email path string true "Email"
// @Success     200 {object} map[string]string
// @Router      /api/v1/marketing/unsubscribe/{listId}/{email} [get]
func (h *MarketingHandler) Unsubscribe(c *gin.Context) {
	listID := c.Param("listId")
	email := c.Param("email")

	var subscriber models.MarketingSubscriber
	if err := h.db.Where("list_id = ? AND email = ?", listID, email).First(&subscriber).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Subscriber not found"})
		return
	}

	now := time.Now()
	h.db.Model(&subscriber).Updates(map[string]interface{}{
		"status":           models.SubscriberStatusUnsubscribed,
		"unsubscribed_at":  &now,
	})

	// Update list count
	h.db.Model(&models.MarketingList{}).Where("id = ?", listID).
		Update("subscriber_count", gorm.Expr("GREATEST(subscriber_count - 1, 0)"))

	c.JSON(http.StatusOK, gin.H{"message": "Successfully unsubscribed"})
}

// @Summary     Get campaign statistics
// @Tags        marketing
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Campaign ID"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/marketing/campaigns/{id}/stats [get]
func (h *MarketingHandler) CampaignStats(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)
	id := c.Param("id")

	var campaign models.MarketingCampaign
	if err := h.db.Where("id = ? AND user_id = ?", id, userID).First(&campaign).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Campaign not found"})
		return
	}

	// Get event counts
	var events []struct {
		EventType string `json:"event_type"`
		Count     int    `json:"count"`
	}
	h.db.Model(&models.CampaignEvent{}).
		Where("campaign_id = ?", campaign.ID).
		Select("event_type, count(*) as count").
		Group("event_type").
		Find(&events)

	stats := gin.H{
		"campaign_id":      campaign.ID,
		"total_recipients": campaign.TotalRecipients,
		"sent":             campaign.SentCount,
		"opens":            campaign.OpenCount,
		"clicks":           campaign.ClickCount,
		"bounces":          campaign.BounceCount,
		"unsubscribes":     campaign.UnsubCount,
		"spam_reports":     campaign.SpamCount,
	}

	if campaign.SentCount > 0 {
		stats["open_rate"] = float64(campaign.OpenCount) / float64(campaign.SentCount) * 100
		stats["click_rate"] = float64(campaign.ClickCount) / float64(campaign.SentCount) * 100
		stats["bounce_rate"] = float64(campaign.BounceCount) / float64(campaign.SentCount) * 100
	}

	stats["events"] = events
	c.JSON(http.StatusOK, stats)
}

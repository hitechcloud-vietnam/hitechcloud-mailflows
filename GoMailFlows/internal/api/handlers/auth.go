package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/api/middleware"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/config"
	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewAuthHandler(db *gorm.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, cfg: cfg}
}

// ============================================================
// Request/Response DTOs
// ============================================================

type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password" binding:"required"`
}

type TokenResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	TokenType    string       `json:"token_type"`
	User         UserResponse `json:"user"`
}

type UserResponse struct {
	ID               string  `json:"id"`
	Email            string  `json:"email"`
	Username         string  `json:"username"`
	DisplayName      string  `json:"displayName"`
	FirstName        string  `json:"first_name"`
	LastName         string  `json:"last_name"`
	Role             string  `json:"role"`
	Status           string  `json:"status"`
	IsAdmin          bool    `json:"isAdmin"`
	IsEmailVerified  bool    `json:"is_email_verified"`
	MFAEnabled       bool    `json:"mfa_enabled"`
	TotpEnabled      bool    `json:"totpEnabled"`
	PackageID        *string `json:"package_id,omitempty"`
	StorageUsed      int64   `json:"storage_used"`
	StorageQuota     int64   `json:"storage_quota"`
	MaxEmailAccounts int     `json:"max_email_accounts"`
}

func toUserResponse(u *models.User) UserResponse {
	displayName := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if displayName == "" {
		displayName = u.Email
	}
	resp := UserResponse{
		ID:               u.ID.String(),
		Email:            u.Email,
		Username:         u.Email,
		DisplayName:      displayName,
		FirstName:        u.FirstName,
		LastName:         u.LastName,
		Role:             string(u.Role),
		Status:           string(u.Status),
		IsAdmin:          u.Role == models.RoleAdmin,
		IsEmailVerified:  u.IsEmailVerified,
		MFAEnabled:       u.MFAEnabled,
		TotpEnabled:      u.MFAEnabled,
		StorageUsed:      u.StorageUsed,
		StorageQuota:     u.StorageQuota,
		MaxEmailAccounts: u.MaxEmailAccounts,
	}
	if u.PackageID != nil {
		pid := u.PackageID.String()
		resp.PackageID = &pid
	}
	return resp
}

// ============================================================
// Register
// ============================================================

// @Summary     Register a new user
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body RegisterRequest true "Registration details"
// @Success     201 {object} TokenResponse
// @Failure     400 {object} map[string]string
// @Failure     409 {object} map[string]string
// @Router      /api/v1/auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check existing user
	var existing models.User
	if err := h.db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         models.RoleUser,
		Status:       models.UserStatusActive,
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate tokens
	tokenResp, err := h.generateTokens(&user, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	// Log auth event
	h.logAuthEvent(&user.ID, models.AuthEventRegister, c, true, "")

	c.JSON(http.StatusCreated, tokenResp)
}

// ============================================================
// Login
// ============================================================

// @Summary     Login user
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body LoginRequest true "Login credentials"
// @Success     200 {object} TokenResponse
// @Failure     401 {object} map[string]string
// @Router      /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	loginIdentifier := strings.ToLower(strings.TrimSpace(req.Email))
	if loginIdentifier == "" {
		loginIdentifier = strings.ToLower(strings.TrimSpace(req.Username))
	}
	if loginIdentifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username or email is required"})
		return
	}

	var user models.User
	if err := h.db.Where("LOWER(email) = ?", loginIdentifier).First(&user).Error; err != nil {
		h.logAuthEvent(nil, models.AuthEventLogin, c, false, "user not found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if user.Status != models.UserStatusActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is " + string(user.Status)})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		h.logAuthEvent(&user.ID, models.AuthEventLogin, c, false, "invalid password")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Update last login
	now := time.Now()
	h.db.Model(&user).Updates(map[string]interface{}{
		"last_login_at": &now,
		"last_login_ip": c.ClientIP(),
	})

	tokenResp, err := h.generateTokens(&user, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	h.logAuthEvent(&user.ID, models.AuthEventLogin, c, true, "")

	// Set httpOnly session cookie for web client
	c.SetCookie("mf_token", tokenResp.AccessToken, 7*24*3600, "/", "", false, true)

	userResp := toUserResponse(&user)
	c.JSON(http.StatusOK, gin.H{
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"expires_in":    tokenResp.ExpiresIn,
		"token_type":    tokenResp.TokenType,
		"user":          userResp,
	})
}

// ============================================================
// Refresh Token
// ============================================================

// @Summary     Refresh access token
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       body body map[string]string true "Refresh token"
// @Success     200 {object} TokenResponse
// @Failure     401 {object} map[string]string
// @Router      /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenHash := hashToken(req.RefreshToken)
	var stored models.RefreshToken
	if err := h.db.Where("token_hash = ? AND revoked = false AND expires_at > ?",
		tokenHash, time.Now()).First(&stored).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	var user models.User
	if err := h.db.First(&user, stored.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Revoke old token
	h.db.Model(&stored).Update("revoked", true)

	tokenResp, err := h.generateTokens(&user, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate tokens"})
		return
	}

	h.logAuthEvent(&user.ID, models.AuthEventTokenRefresh, c, true, "")
	c.JSON(http.StatusOK, tokenResp)
}

// ============================================================
// Get Current User
// ============================================================

// @Summary     Get current user profile
// @Tags        auth
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} UserResponse
// @Failure     401 {object} map[string]string
// @Router      /api/v1/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	userResp := toUserResponse(&user)
	c.JSON(http.StatusOK, gin.H{"user": userResp})
}

// ============================================================
// Logout
// ============================================================

// @Summary     Logout user
// @Tags        auth
// @Security    BearerAuth
// @Success     200 {object} map[string]string
// @Router      /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	// Revoke all refresh tokens for user
	h.db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked = false", userID).
		Update("revoked", true)

	c.SetCookie("mf_token", "", -1, "/", "", false, true)

	h.logAuthEvent(&userID, models.AuthEventLogout, c, true, "")
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// ============================================================
// Change Password
// ============================================================

// @Summary     Change password
// @Tags        auth
// @Accept      json
// @Security    BearerAuth
// @Param       body body map[string]string true "Old and new password"
// @Success     200 {object} map[string]string
// @Router      /api/v1/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.MustGet("user_id").(uuid.UUID)

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid current password"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	h.db.Model(&user).Update("password_hash", string(hash))

	// Revoke all tokens
	h.db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked = false", userID).
		Update("revoked", true)

	h.logAuthEvent(&userID, models.AuthEventPasswordChange, c, true, "")
	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// ============================================================
// Helper Methods
// ============================================================

func (h *AuthHandler) generateTokens(user *models.User, ip, userAgent string) (*TokenResponse, error) {
	// Access token
	accessExpiry := time.Now().Add(h.cfg.JWT.AccessTokenTTL)
	accessClaims := &middleware.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    h.cfg.JWT.Issuer,
			Subject:   user.ID.String(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(h.cfg.JWT.Secret))
	if err != nil {
		return nil, err
	}

	// Refresh token
	refreshTokenBytes := make([]byte, 32)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		return nil, err
	}
	refreshTokenString := hex.EncodeToString(refreshTokenBytes)

	refreshToken := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashToken(refreshTokenString),
		ExpiresAt: time.Now().Add(h.cfg.JWT.RefreshTokenTTL),
		UserAgent: userAgent,
		IPAddress: ip,
	}
	if err := h.db.Create(&refreshToken).Error; err != nil {
		return nil, err
	}

	return &TokenResponse{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int(h.cfg.JWT.AccessTokenTTL.Seconds()),
		TokenType:    "Bearer",
		User:         toUserResponse(user),
	}, nil
}

func (h *AuthHandler) logAuthEvent(userID *uuid.UUID, event models.AuthEventType, c *gin.Context, success bool, details string) {
	authEvent := models.AuthEvent{
		UserID:    userID,
		Event:     event,
		IPAddress: c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Success:   success,
		Details:   details,
	}
	h.db.Create(&authEvent)
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

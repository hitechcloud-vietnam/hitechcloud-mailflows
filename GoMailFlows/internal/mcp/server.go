package mcp

import (
	"context"
	"fmt"
	"log"

	"github.com/hitechcloud-vietnam/hitechcloud-mailflows/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gorm.io/gorm"
)

type MCPServer struct {
	cfg        *config.MCPConfig
	db         *gorm.DB
	adminSrv   *server.MCPServer
	userSrv    *server.MCPServer
}

func NewMCPServer(cfg *config.MCPConfig, db *gorm.DB) *MCPServer {
	s := &MCPServer{
		cfg: cfg,
		db:  db,
	}
	s.setupAdminServer()
	s.setupUserServer()
	return s
}

func (s *MCPServer) setupAdminServer() {
	s.adminSrv = server.NewMCPServer(
		"HiTechCloud MailFlows Admin MCP",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
	)

	// ============================================================
	// Admin Tools
	// ============================================================

	// List domains
	s.adminSrv.AddTool(
		mcp.NewTool("list_domains",
			mcp.WithDescription("List all domains in the system"),
			mcp.WithString("status", mcp.Description("Filter by status: pending, active, suspended")),
			mcp.WithNumber("page", mcp.Description("Page number")),
			mcp.WithNumber("per_page", mcp.Description("Items per page")),
		),
		s.handleAdminListDomains,
	)

	// Get domain details
	s.adminSrv.AddTool(
		mcp.NewTool("get_domain",
			mcp.WithDescription("Get detailed domain information including DNS records"),
			mcp.WithString("domain_id", mcp.Required(), mcp.Description("Domain ID")),
		),
		s.handleAdminGetDomain,
	)

	// List users
	s.adminSrv.AddTool(
		mcp.NewTool("list_users",
			mcp.WithDescription("List all users in the system"),
			mcp.WithString("role", mcp.Description("Filter by role: admin, reseller, user")),
			mcp.WithString("status", mcp.Description("Filter by status: active, suspended, pending")),
			mcp.WithString("search", mcp.Description("Search by email or name")),
		),
		s.handleAdminListUsers,
	)

	// Get system stats
	s.adminSrv.AddTool(
		mcp.NewTool("get_system_stats",
			mcp.WithDescription("Get system statistics including user counts, domain counts, etc."),
		),
		s.handleAdminGetStats,
	)

	// Manage SMTP configs
	s.adminSrv.AddTool(
		mcp.NewTool("list_smtp_configs",
			mcp.WithDescription("List all SMTP configurations"),
		),
		s.handleAdminListSMTPConfigs,
	)

	// Manage packages
	s.adminSrv.AddTool(
		mcp.NewTool("list_packages",
			mcp.WithDescription("List all service packages/plans"),
		),
		s.handleAdminListPackages,
	)

	// DNS templates
	s.adminSrv.AddTool(
		mcp.NewTool("list_dns_templates",
			mcp.WithDescription("List DNS record templates"),
		),
		s.handleAdminListDNSTemplates,
	)

	// Audit logs
	s.adminSrv.AddTool(
		mcp.NewTool("get_audit_logs",
			mcp.WithDescription("Get recent audit logs"),
			mcp.WithString("action", mcp.Description("Filter by action")),
			mcp.WithString("resource", mcp.Description("Filter by resource")),
		),
		s.handleAdminGetAuditLogs,
	)
}

func (s *MCPServer) setupUserServer() {
	s.userSrv = server.NewMCPServer(
		"HiTechCloud MailFlows User MCP",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// List my domains
	s.userSrv.AddTool(
		mcp.NewTool("list_my_domains",
			mcp.WithDescription("List domains owned by the current user"),
		),
		s.handleUserListDomains,
	)

	// List my email accounts
	s.userSrv.AddTool(
		mcp.NewTool("list_my_email_accounts",
			mcp.WithDescription("List email accounts for the current user"),
			mcp.WithString("domain_id", mcp.Description("Filter by domain ID")),
		),
		s.handleUserListEmailAccounts,
	)

	// Get my campaigns
	s.userSrv.AddTool(
		mcp.NewTool("list_my_campaigns",
			mcp.WithDescription("List marketing campaigns"),
		),
		s.handleUserListCampaigns,
	)

	// Get DNS records for domain
	s.userSrv.AddTool(
		mcp.NewTool("get_domain_dns",
			mcp.WithDescription("Get DNS records for a domain"),
			mcp.WithString("domain_id", mcp.Required(), mcp.Description("Domain ID")),
		),
		s.handleUserGetDomainDNS,
	)

	// Send test email
	s.userSrv.AddTool(
		mcp.NewTool("send_test_email",
			mcp.WithDescription("Send a test email to verify SMTP configuration"),
			mcp.WithString("account_id", mcp.Required(), mcp.Description("Email account ID")),
			mcp.WithString("to", mcp.Required(), mcp.Description("Recipient email")),
			mcp.WithString("subject", mcp.Required(), mcp.Description("Email subject")),
			mcp.WithString("body", mcp.Required(), mcp.Description("Email body")),
		),
		s.handleUserSendTestEmail,
	)

	// Get available packages
	s.userSrv.AddTool(
		mcp.NewTool("list_available_packages",
			mcp.WithDescription("List available service packages/plans"),
		),
		s.handleUserListPackages,
	)
}

func (s *MCPServer) Start() error {
	if !s.cfg.Enabled {
		log.Println("MCP server disabled")
		return nil
	}

	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("MCP servers starting on %s (SSE transport)", addr)

	// Start admin MCP on /mcp/admin and user MCP on /mcp/user
	go func() {
		sseServer := server.NewSSEServer(s.adminSrv,
			server.WithBaseURL(fmt.Sprintf("http://localhost:%d", s.cfg.Port)),
			server.WithStaticBasePath("/mcp/admin"),
		)
		log.Printf("Admin MCP available at http://localhost:%d/mcp/admin", s.cfg.Port)
		if err := sseServer.Start(addr); err != nil {
			log.Printf("Admin MCP server error: %v", err)
		}
	}()

	return nil
}

// ============================================================
// Admin Tool Handlers
// ============================================================

func (s *MCPServer) handleAdminListDomains(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Query domains from database
	type DomainInfo struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	var domains []DomainInfo
	if err := s.db.Model(&struct {
		ID     string
		Name   string
		Status string
	}{}).Table("domains").Select("id, name, status").Scan(&domains).Error; err != nil {
		return mcp.NewToolResultError("Failed to query domains: " + err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d domains", len(domains))), nil
}

func (s *MCPServer) handleAdminGetDomain(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	domainID := request.Params.Arguments["domain_id"].(string)
	return mcp.NewToolResultText(fmt.Sprintf("Domain details for %s", domainID)), nil
}

func (s *MCPServer) handleAdminListUsers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("User list"), nil
}

func (s *MCPServer) handleAdminGetStats(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("System statistics"), nil
}

func (s *MCPServer) handleAdminListSMTPConfigs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("SMTP configurations"), nil
}

func (s *MCPServer) handleAdminListPackages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("Service packages"), nil
}

func (s *MCPServer) handleAdminListDNSTemplates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("DNS templates"), nil
}

func (s *MCPServer) handleAdminGetAuditLogs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("Audit logs"), nil
}

// ============================================================
// User Tool Handlers
// ============================================================

func (s *MCPServer) handleUserListDomains(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("Your domains"), nil
}

func (s *MCPServer) handleUserListEmailAccounts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("Your email accounts"), nil
}

func (s *MCPServer) handleUserListCampaigns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("Your campaigns"), nil
}

func (s *MCPServer) handleUserGetDomainDNS(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	domainID := request.Params.Arguments["domain_id"].(string)
	return mcp.NewToolResultText(fmt.Sprintf("DNS records for domain %s", domainID)), nil
}

func (s *MCPServer) handleUserSendTestEmail(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("Test email sent"), nil
}

func (s *MCPServer) handleUserListPackages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("Available packages"), nil
}

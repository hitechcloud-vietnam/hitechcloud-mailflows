# HiTechCloud MailFlows - Go Backend

Enterprise Email SaaS Platform built with Go.

## Architecture

```
GoMailFlows/
├── cmd/server/              # Main API server entry point
├── internal/
│   ├── api/
│   │   ├── handlers/        # HTTP request handlers
│   │   ├── middleware/       # Auth, CORS, logging middleware
│   │   └── router/          # API route definitions
│   ├── config/              # Configuration management
│   ├── database/            # Database connection & migrations
│   ├── jmap/                # JMAP server (RFC 8620/8621)
│   ├── mcp/                 # MCP server (Model Context Protocol)
│   ├── models/              # Data models & DTOs
│   └── smtp/                # SMTP server
├── configs/                 # Configuration files
├── contrib/                 # Systemd service files
├── docs/                    # OpenAPI specifications
└── Makefile                 # Build & deployment targets
```

## Features

### Core Email
- **SMTP Server**: Full SMTP server for receiving and sending email
- **IMAP Support**: IMAP protocol for email access
- **JMAP Support**: Modern JMAP protocol (RFC 8620/8621)
- **WebMail**: React-based webmail interface (existing frontend)

### Domain Management
- Domain registration and verification
- Automatic DNS record provisioning (MX, SPF, DKIM, DMARC, MTA-STS, TLSRPT)
- DKIM key generation (Ed25519)
- DNS template system for standardized provisioning
- Autoconfig/Autodiscover support (RFC 6186/6764)
- SRV records for IMAP, SMTP, JMAP auto-discovery

### SMTP Configuration
- Per-domain SMTP relay configuration
- Global SMTP fallback
- Multiple SMTP configs per system
- Connection testing
- Rate limiting per account (daily/hourly)

### SaaS Packages
- Flexible service plans (Free, Starter, Professional, Enterprise)
- Granular feature control (30+ features)
- Feature groups for easy configuration
- Per-package limits (domains, mailboxes, storage, sends)
- Public/private package visibility

### Email Marketing
- Campaign management (create, schedule, send)
- Subscriber list management
- Bulk subscriber import
- Campaign statistics (opens, clicks, bounces)
- Unsubscribe management
- A/B testing (Enterprise)

### Admin Panel
- Dashboard with system statistics
- User management (CRUD, suspend, activate)
- SMTP configuration management
- DNS template management
- System settings
- Audit logging

### Security
- JWT authentication with refresh tokens
- Role-based access control (Admin, Reseller, User)
- MFA support (TOTP)
- Rate limiting
- CORS configuration
- Security headers
- Password hashing (bcrypt)
- Auth event logging

### API & Integration
- **OpenAPI 3.1** specification
- **Swagger UI** at `/swagger/index.html`
- **MCP Server** for AI-powered management
- RESTful API design
- Pagination support
- Filter/search capabilities

## Quick Start

### Prerequisites
- Go 1.23+
- PostgreSQL 16+
- Redis 7+

### Development

```bash
# Start dependencies (Docker - dev only)
docker-compose -f docker-compose.dev.yml up -d

# Run the application
make run

# Or directly
go run ./cmd/server
```

### Production (Linux Bare Metal)

```bash
# Build for Linux
make build-linux

# Install to /opt/mailflows
make install

# Start service
sudo systemctl start mailflows
sudo systemctl status mailflows
```

### API Documentation

Once running, visit:
- Swagger UI: `http://localhost:8080/swagger/index.html`
- OpenAPI spec: `http://localhost:8080/openapi.yaml`

### MCP Server

The MCP server provides AI-powered management tools:
- Admin MCP: `http://localhost:8082/mcp/admin`
- User MCP: `http://localhost:8082/mcp/user`

## Configuration

Configuration is loaded from:
1. `./config.yaml`
2. `./configs/config.yaml`
3. `/etc/mailflows/config.yaml`
4. Environment variables (prefix: `MF_`)

### Key Configuration

```yaml
server:
  port: 8080
  environment: dev  # dev, staging, production

database:
  host: localhost
  port: 5432
  user: mailflows
  password: mailflows
  name: mailflows

jwt:
  secret: "your-secret-key"
  access_token_ttl: 15m
  refresh_token_ttl: 168h

smtp:
  port: 25
  tls_port: 587

jmap:
  port: 8081

mcp:
  enabled: true
  port: 8082
```

## API Endpoints

### Authentication
- `POST /api/v1/auth/register` - Register
- `POST /api/v1/auth/login` - Login
- `POST /api/v1/auth/refresh` - Refresh token
- `GET /api/v1/auth/me` - Get profile
- `POST /api/v1/auth/logout` - Logout

### Domains
- `GET /api/v1/domains` - List domains
- `POST /api/v1/domains` - Create domain
- `GET /api/v1/domains/:id` - Get domain
- `DELETE /api/v1/domains/:id` - Delete domain
- `POST /api/v1/domains/:id/verify` - Verify domain
- `POST /api/v1/domains/:id/dns/regenerate` - Regenerate DNS

### Email Accounts
- `GET /api/v1/email-accounts` - List accounts
- `POST /api/v1/email-accounts` - Create account
- `PUT /api/v1/email-accounts/:id` - Update account
- `DELETE /api/v1/email-accounts/:id` - Delete account

### Mail
- `GET /api/v1/mail/folders` - List folders
- `GET /api/v1/mail/messages` - List messages
- `POST /api/v1/mail/send` - Send email

### Marketing
- `GET /api/v1/marketing/campaigns` - List campaigns
- `POST /api/v1/marketing/campaigns` - Create campaign
- `POST /api/v1/marketing/campaigns/:id/send` - Send campaign
- `GET /api/v1/marketing/campaigns/:id/stats` - Campaign stats
- `GET /api/v1/marketing/lists` - List subscriber lists
- `POST /api/v1/marketing/subscribers/bulk` - Bulk add subscribers

### Admin (Admin only)
- `GET /api/v1/admin/dashboard` - Dashboard stats
- `GET /api/v1/admin/users` - List users
- `GET /api/v1/admin/smtp-configs` - SMTP configs
- `GET /api/v1/admin/dns-templates` - DNS templates
- `GET /api/v1/admin/settings` - System settings
- `GET /api/v1/admin/audit-logs` - Audit logs

## Deployment

### Systemd Service

```bash
# Install
make install

# Manage
sudo systemctl start mailflows
sudo systemctl stop mailflows
sudo systemctl restart mailflows
sudo journalctl -u mailflows -f
```

### Manual Deployment

```bash
# Build
make build-linux

# Copy binary
cp bin/mailflows-linux-amd64 /usr/local/bin/mailflows

# Copy config
cp configs/config.yaml /etc/mailflows/config.yaml

# Run
mailflows
```

## License

AGPL-3.0 (free personal use) + Commercial license available

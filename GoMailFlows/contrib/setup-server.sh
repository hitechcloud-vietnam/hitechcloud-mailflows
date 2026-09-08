#!/bin/bash
# ============================================================
# HiTechCloud MailFlows - Server Setup Script
# Run this once on the production server (116.118.2.52)
# ============================================================

set -e

DEPLOY_PATH="/opt/mailflows"
DOMAIN="mail.photuegroup.vn"

echo "=========================================="
echo "HiTechCloud MailFlows - Server Setup"
echo "=========================================="

# ---- System packages ----
echo ">>> Installing system dependencies..."
apt-get update -qq
apt-get install -y -qq postgresql redis-server curl wget certbot

# ---- PostgreSQL ----
echo ">>> Setting up PostgreSQL..."
systemctl enable postgresql
systemctl start postgresql

# Create database and user
sudo -u postgres psql -c "CREATE USER mailflows WITH PASSWORD 'mailflows_secure_2024';" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE mailflows OWNER mailflows;" 2>/dev/null || true
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE mailflows TO mailflows;" 2>/dev/null || true

# ---- Redis ----
echo ">>> Setting up Redis..."
systemctl enable redis-server
systemctl start redis-server

# ---- Create directories ----
echo ">>> Creating application directories..."
mkdir -p ${DEPLOY_PATH}/{bin,configs,logs,docs,web/static,web/templates}
mkdir -p /etc/mailflows

# ---- Create system user ----
echo ">>> Creating mailflows system user..."
useradd -r -s /bin/false mailflows 2>/dev/null || true

# ---- Set permissions ----
chown -R mailflows:mailflows ${DEPLOY_PATH}

# ---- Firewall ----
echo ">>> Configuring firewall..."
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP
ufw allow 443/tcp   # HTTPS
ufw allow 25/tcp    # SMTP
ufw allow 587/tcp   # SMTP Submission
ufw allow 993/tcp   # IMAPS
ufw allow 143/tcp   # IMAP
ufw allow 8080/tcp  # API
ufw allow 8081/tcp  # JMAP
ufw --force enable

# ---- Nginx/Caddy for reverse proxy ----
echo ">>> Installing Caddy..."
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg 2>/dev/null
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
apt-get update -qq
apt-get install -y -qq caddy

# ---- Caddyfile ----
echo ">>> Configuring Caddy..."
cat > /etc/caddy/Caddyfile << 'CADDYEOF'
mail.photuegroup.vn {
    # TLS with automatic Let's Encrypt
    tls {
        protocols tls1.2 tls1.3
    }

    # Security headers
    header {
        X-Content-Type-Options nosniff
        X-Frame-Options SAMEORIGIN
        Referrer-Policy strict-origin-when-cross-origin
        X-XSS-Protection "1; mode=block"
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        -Server
    }

    # Frontend static files
    root * /opt/mailflows/web/static
    file_server

    # API proxy
    handle /api/* {
        reverse_proxy localhost:8080
    }

    # Swagger proxy
    handle /swagger/* {
        reverse_proxy localhost:8080
    }

    handle /openapi.yaml {
        reverse_proxy localhost:8080
    }

    handle /openapi.json {
        reverse_proxy localhost:8080
    }

    # Health check
    handle /health {
        reverse_proxy localhost:8080
    }

    # WebSocket proxy
    handle /ws/* {
        reverse_proxy localhost:8080 {
            header_up Connection {>Connection}
            header_up Upgrade {>Upgrade}
        }
    }

    # JMAP proxy
    handle /jmap/* {
        reverse_proxy localhost:8081
    }

    handle /.well-known/jmap {
        reverse_proxy localhost:8081
    }

    # SPA fallback
    try_files {path} /index.html
}
CADDYEOF

# ---- Systemd service ----
echo ">>> Installing systemd service..."
if [ -f ${DEPLOY_PATH}/contrib/mailflows.service ]; then
    cp ${DEPLOY_PATH}/contrib/mailflows.service /etc/systemd/system/mailflows.service
fi

systemctl daemon-reload
systemctl enable caddy
systemctl restart caddy

# ---- Log rotation ----
echo ">>> Setting up log rotation..."
cat > /etc/logrotate.d/mailflows << 'LOGEOF'
/opt/mailflows/logs/*.log {
    daily
    rotate 14
    compress
    delaycompress
    notifempty
    create 0640 mailflows mailflows
    sharedscripts
    postrotate
        systemctl reload mailflows 2>/dev/null || true
    endscript
}
LOGEOF

echo ""
echo "=========================================="
echo "Server setup complete!"
echo "=========================================="
echo ""
echo "Domain: ${DOMAIN}"
echo "Deploy path: ${DEPLOY_PATH}"
echo "Database: PostgreSQL (mailflows/mailflows)"
echo "Cache: Redis"
echo "Reverse proxy: Caddy (auto TLS)"
echo ""
echo "Next steps:"
echo "1. Deploy the application binary via CI/CD"
echo "2. Configure DNS A record for ${DOMAIN} -> $(curl -s ifconfig.me)"
echo "3. Caddy will auto-provision TLS certificate"
echo ""

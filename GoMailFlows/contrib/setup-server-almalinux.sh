#!/bin/bash
# ============================================================
# HiTechCloud MailFlows - Server Setup Script for RHEL/AlmaLinux/Rocky 8+
# Target OS: AlmaLinux 8.10
# Domain: mail.photuegroup.vn
# ============================================================

set -e

DEPLOY_PATH="/opt/mailflows"
DOMAIN="mail.photuegroup.vn"

echo "=========================================="
echo "HiTechCloud MailFlows - Server Setup (AlmaLinux 8)"
echo "=========================================="

# ---- Disable SELinux or set permissive ----
if [ -f /etc/selinux/config ]; then
    setenforce 0 || true
    sed -i 's/^SELINUX=.*/SELINUX=permissive/' /etc/selinux/config
fi

# ---- EPEL & Repos ----
echo ">>> Enabling EPEL repo and DNF modules..."
dnf install -y -q epel-release
dnf install -y -q curl wget tar gzip jq rsync bind-utils

# ---- PostgreSQL 16 ----
echo ">>> Installing PostgreSQL 16..."
if ! command -v psql &> /dev/null; then
    dnf install -y -q https://download.postgresql.org/pub/repos/yum/reporpms/EL-8-x86_64/pgdg-redhat-repo-latest.noarch.rpm || true
    dnf -qy module disable postgresql || true
    dnf install -y -q postgresql16-server postgresql16 || dnf install -y -q postgresql-server postgresql
    
    # Init DB if not initialized
    if [ -f /usr/pgsql-16/bin/postgresql-16-setup ]; then
        /usr/pgsql-16/bin/postgresql-16-setup initdb || true
        systemctl enable postgresql-16
        systemctl start postgresql-16
    else
        postgresql-setup --initdb || true
        systemctl enable postgresql
        systemctl start postgresql
    fi
else
    echo "PostgreSQL already installed."
    systemctl start postgresql-16 2>/dev/null || systemctl start postgresql 2>/dev/null || true
fi

# Configure postgres auth and db
echo ">>> Configuring PostgreSQL user and database..."
sudo -u postgres psql -c "CREATE USER mailflows WITH PASSWORD 'mailflows';" 2>/dev/null || true
sudo -u postgres psql -c "ALTER USER mailflows WITH PASSWORD 'mailflows';" 2>/dev/null || true
sudo -u postgres psql -c "CREATE DATABASE mailflows OWNER mailflows;" 2>/dev/null || true
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE mailflows TO mailflows;" 2>/dev/null || true

# ---- Redis ----
echo ">>> Installing and configuring Redis..."
dnf install -y -q redis
systemctl enable redis
systemctl start redis

# ---- Nginx ----
echo ">>> Installing Nginx and Certbot..."
dnf install -y -q nginx certbot python3-certbot-nginx
systemctl enable nginx

# ---- Create directories ----
echo ">>> Creating application directories..."
mkdir -p ${DEPLOY_PATH}/{bin,configs,logs,docs,web/static}
mkdir -p /etc/mailflows

# ---- System user ----
echo ">>> Creating mailflows user..."
id -u mailflows &>/dev/null || useradd -r -s /bin/false mailflows
chown -R mailflows:mailflows ${DEPLOY_PATH}

# ---- Firewall ----
echo ">>> Configuring firewalld..."
if systemctl is-active firewalld &>/dev/null; then
    firewall-cmd --permanent --add-service=http || true
    firewall-cmd --permanent --add-service=https || true
    firewall-cmd --permanent --add-port=25/tcp || true
    firewall-cmd --permanent --add-port=587/tcp || true
    firewall-cmd --permanent --add-port=465/tcp || true
    firewall-cmd --permanent --add-port=143/tcp || true
    firewall-cmd --permanent --add-port=993/tcp || true
    firewall-cmd --permanent --add-port=8080/tcp || true
    firewall-cmd --permanent --add-port=8081/tcp || true
    firewall-cmd --reload || true
fi

# ---- Nginx vhost for mail.photuegroup.vn ----
echo ">>> Configuring Nginx for ${DOMAIN}..."
cat > /etc/nginx/conf.d/mailflows.conf << 'EOF'
server {
    listen 80;
    listen [::]:80;
    server_name mail.photuegroup.vn;

    root /opt/mailflows/web/static;
    index index.html;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # API Proxy to Go backend
    location /api/ {
        proxy_pass http://127.0.0.1:8080/api/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 90;
    }

    # Swagger / OpenAPI
    location /swagger/ {
        proxy_pass http://127.0.0.1:8080/swagger/;
        proxy_set_header Host $host;
    }

    location /openapi.yaml {
        proxy_pass http://127.0.0.1:8080/openapi.yaml;
        proxy_set_header Host $host;
    }

    location /openapi.json {
        proxy_pass http://127.0.0.1:8080/openapi.json;
        proxy_set_header Host $host;
    }

    # Health check
    location /health {
        proxy_pass http://127.0.0.1:8080/health;
        proxy_set_header Host $host;
    }

    # JMAP Protocol proxy
    location /.well-known/jmap {
        proxy_pass http://127.0.0.1:8081/.well-known/jmap;
        proxy_set_header Host $host;
    }

    location /jmap/ {
        proxy_pass http://127.0.0.1:8081/jmap/;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }

    # Static assets & SPA fallback
    location / {
        try_files $uri $uri/ /index.html;
    }
}
EOF

# Test nginx config and restart
nginx -t && systemctl restart nginx

echo ">>> Server environment setup complete!"
echo "Database: PostgreSQL running"
echo "Cache: Redis running"
echo "Web Server: Nginx running on port 80"

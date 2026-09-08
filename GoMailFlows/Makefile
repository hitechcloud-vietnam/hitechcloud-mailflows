# HiTechCloud MailFlows - Makefile
# ============================================================

APP_NAME    := mailflows
VERSION     := 1.0.0
BUILD_DIR   := ./bin
GO          := go
GOTEST      := $(GO) test
GOBUILD     := $(GO) build
GOMOD       := $(GO) mod
LDFLAGS     := -ldflags "-s -w -X main.version=$(VERSION)"

# Default target
.PHONY: all
all: build

# ============================================================
# Build
# ============================================================

.PHONY: build
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

.PHONY: build-linux
build-linux:
	@echo "Building $(APP_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./cmd/server
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)-linux-amd64"

.PHONY: build-all
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 ./cmd/server
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 ./cmd/server
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/server
	@echo "All builds complete"

# ============================================================
# Development
# ============================================================

.PHONY: run
run:
	$(GO) run ./cmd/server

.PHONY: dev
dev:
	@echo "Starting development environment..."
	@docker-compose -f docker-compose.dev.yml up -d postgres redis
	@sleep 3
	$(GO) run ./cmd/server

.PHONY: test
test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

.PHONY: test-short
test-short:
	$(GOTEST) -short ./...

.PHONY: coverage
coverage: test
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: lint
lint:
	golangci-lint run ./...

# ============================================================
# Dependencies
# ============================================================

.PHONY: deps
deps:
	$(GOMOD) download
	$(GOMOD) tidy

.PHONY: deps-update
deps-update:
	$(GOMOD) get -u ./...
	$(GOMOD) tidy

# ============================================================
# Documentation / OpenAPI
# ============================================================

.PHONY: swagger
swagger:
	@echo "Generating Swagger docs..."
	swag init -g cmd/server/main.go -o docs
	@echo "Swagger docs generated in docs/"

.PHONY: openapi-validate
openapi-validate:
	@echo "Validating OpenAPI spec..."
	npx @redocly/cli lint docs/openapi.yaml

# ============================================================
# Database
# ============================================================

.PHONY: db-create
db-create:
	@echo "Creating database..."
	psql -h localhost -U postgres -c "CREATE USER mailflows WITH PASSWORD 'mailflows';"
	psql -h localhost -U postgres -c "CREATE DATABASE mailflows OWNER mailflows;"
	psql -h localhost -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE mailflows TO mailflows;"

.PHONY: db-drop
db-drop:
	@echo "Dropping database..."
	psql -h localhost -U postgres -c "DROP DATABASE IF EXISTS mailflows;"
	psql -h localhost -U postgres -c "DROP USER IF EXISTS mailflows;"

.PHONY: db-reset
db-reset: db-drop db-create
	@echo "Database reset complete"

# ============================================================
# Installation (Linux bare metal)
# ============================================================

.PHONY: install
install: build-linux
	@echo "Installing $(APP_NAME)..."
	sudo useradd -r -s /bin/false mailflows 2>/dev/null || true
	sudo mkdir -p /opt/mailflows/{bin,configs,logs}
	sudo cp $(BUILD_DIR)/$(APP_NAME)-linux-amd64 /opt/mailflows/bin/$(APP_NAME)
	sudo cp configs/config.yaml /opt/mailflows/configs/config.yaml
	sudo cp contrib/mailflows.service /etc/systemd/system/mailflows.service
	sudo chown -R mailflows:mailflows /opt/mailflows
	sudo systemctl daemon-reload
	@echo "Installed. Run: sudo systemctl start mailflows"

.PHONY: uninstall
uninstall:
	@echo "Uninstalling $(APP_NAME)..."
	sudo systemctl stop mailflows 2>/dev/null || true
	sudo systemctl disable mailflows 2>/dev/null || true
	sudo rm -f /etc/systemd/system/mailflows.service
	sudo rm -rf /opt/mailflows
	sudo userdel mailflows 2>/dev/null || true
	sudo systemctl daemon-reload
	@echo "Uninstalled"

.PHONY: service-install
service-install:
	sudo cp contrib/mailflows.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable mailflows

.PHONY: service-start
service-start:
	sudo systemctl start mailflows

.PHONY: service-stop
service-stop:
	sudo systemctl stop mailflows

.PHONY: service-status
service-status:
	sudo systemctl status mailflows

.PHONY: service-logs
service-logs:
	sudo journalctl -u mailflows -f

# ============================================================
# Docker (development only)
# ============================================================

.PHONY: docker-dev
docker-dev:
	docker-compose -f docker-compose.dev.yml up --build

.PHONY: docker-down
docker-down:
	docker-compose -f docker-compose.dev.yml down

# ============================================================
# Clean
# ============================================================

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	$(GO) clean -cache -testcache

.PHONY: help
help:
	@echo "HiTechCloud MailFlows - Available targets:"
	@echo ""
	@echo "  build          - Build the application"
	@echo "  build-linux    - Build for Linux"
	@echo "  build-all      - Build for all platforms"
	@echo "  run            - Run the application"
	@echo "  dev            - Start dev environment (Docker DB + app)"
	@echo "  test           - Run tests"
	@echo "  lint           - Run linter"
	@echo "  swagger        - Generate Swagger/OpenAPI docs"
	@echo "  install        - Install to /opt/mailflows (Linux)"
	@echo "  uninstall      - Remove from system"
	@echo "  service-*      - Systemd service management"
	@echo "  docker-dev     - Start with Docker Compose"
	@echo "  clean          - Clean build artifacts"
	@echo "  help           - Show this help"

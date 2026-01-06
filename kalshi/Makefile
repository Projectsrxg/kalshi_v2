.PHONY: build build-linux-arm64 test clean fmt lint vet up down dev

# Build settings
BINARY_DIR := bin
GATHERER_BINARY := $(BINARY_DIR)/gatherer
DEDUPLICATOR_BINARY := $(BINARY_DIR)/deduplicator

# Go settings
GO := go
GOFLAGS := -trimpath
LDFLAGS := -s -w

# Version info (injected at build time)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION_PKG := github.com/rickgao/kalshi-data/internal/version
VERSION_LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).Commit=$(COMMIT) -X $(VERSION_PKG).BuildTime=$(BUILD_TIME)

# Default target
all: build

# Build both binaries for local development
build:
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(GATHERER_BINARY) ./cmd/gatherer
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(DEDUPLICATOR_BINARY) ./cmd/deduplicator

# Build for production (Linux ARM64)
build-linux-arm64:
	@mkdir -p $(BINARY_DIR)
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(GATHERER_BINARY) ./cmd/gatherer
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(DEDUPLICATOR_BINARY) ./cmd/deduplicator

# Run tests
test:
	$(GO) test -race -cover ./...

# Run tests with verbose output
test-verbose:
	$(GO) test -race -cover -v ./...

# Format code
fmt:
	$(GO) fmt ./...

# Run linter
lint:
	golangci-lint run ./...

# Run go vet
vet:
	$(GO) vet ./...

# Clean build artifacts
clean:
	rm -rf $(BINARY_DIR)
	$(GO) clean -cache

# Run gatherer locally
run-gatherer: build
	./$(GATHERER_BINARY) --config configs/gatherer.local.yaml

# Run deduplicator locally
run-deduplicator: build
	./$(DEDUPLICATOR_BINARY) --config configs/deduplicator.local.yaml

# Generate mocks (if using mockgen)
mocks:
	$(GO) generate ./...

# Tidy dependencies
tidy:
	$(GO) mod tidy

# =============================================================================
# Docker commands
# =============================================================================

# Start infrastructure (TimescaleDB, MinIO)
up:
	docker-compose up -d
	@echo "Waiting for TimescaleDB to be ready..."
	@sleep 5
	@docker-compose ps

# Stop infrastructure
down:
	docker-compose down

# Stop and remove volumes (clean slate)
down-clean:
	docker-compose down -v

# Start with monitoring (Prometheus, Grafana)
up-monitoring:
	docker-compose --profile monitoring up -d

# View logs
logs:
	docker-compose logs -f

# Check database is ready
db-ready:
	@docker-compose exec timescaledb pg_isready -U postgres

# Connect to database
db-shell:
	docker-compose exec timescaledb psql -U postgres -d kalshi_ts

# Full local development (start infra + run gatherer)
dev: up
	@echo "Infrastructure started. Run 'make run-gatherer' to start the gatherer."
	@echo ""
	@echo "Or set your Kalshi API credentials first:"
	@echo "  export KALSHI_API_KEY=your-key"
	@echo "  export KALSHI_API_SECRET=your-secret"
	@echo ""
	@echo "Then run:"
	@echo "  make run-gatherer"

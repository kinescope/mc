.PHONY: help proto test test-race test-coverage lint fmt vet build clean install deps

# Variables
GOPATH ?= $(shell go env GOPATH)
PROTO_DIR = proto/cache
PROTO_FILES = $(PROTO_DIR)/*.proto

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

proto: ## Generate protobuf code
	@echo "Generating protobuf code..."
	@protoc -I=. -I=proto -I=$(GOPATH)/src \
		--go_out=$(GOPATH)/src \
		$(PROTO_FILES)
	@echo "Protobuf code generated successfully"

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	@go test -race -v ./...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

lint: ## Run linter (requires golangci-lint)
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install it from https://golangci-lint.run/"; \
	fi

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

build: ## Build the package
	@echo "Building package..."
	@go build ./...

clean: ## Clean generated files
	@echo "Cleaning generated files..."
	@rm -f coverage.out coverage.html
	@go clean ./...

install: ## Install dependencies
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

deps: install ## Alias for install

check: fmt vet lint test ## Run all checks (format, vet, lint, test)

ci: test-race test-coverage lint ## Run CI checks (race detector, coverage, lint)

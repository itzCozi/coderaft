# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Version information
VERSION=1.0
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date +%Y-%m-%d)
LDFLAGS=-ldflags "-X coderaft/internal/commands.Version=$(VERSION) -X coderaft/internal/commands.CommitHash=$(GIT_COMMIT)"

# Binary name
BINARY_NAME=coderaft
BINARY_PATH=./cmd/coderaft

# Build directory
BUILD_DIR=./build

# Default target
all: clean deps build

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build the binary
build:
	@echo "Building $(BINARY_NAME) version $(VERSION) (commit $(GIT_COMMIT))..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

# Install the binary to /usr/local/bin
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)
	@echo "Installation complete. You can now use '$(BINARY_NAME)' from anywhere."

# Build for development (current OS/arch)
dev:
	@echo "Building $(BINARY_NAME) for development..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)"

# Run tests
test:
	$(GOTEST) -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Format code
fmt:
	$(GOCMD) fmt ./...

# Run linter
lint:
	golangci-lint run

# Check formatting
check-fmt:
	@if [ "$(shell gofmt -s -l . | wc -l)" -gt 0 ]; then \
		echo "The following files are not formatted:"; \
		gofmt -s -l .; \
		echo "Please run 'make fmt' to format your code."; \
		exit 1; \
	fi

# Run all quality checks
quality: check-fmt lint
	@echo "Running go vet..."
	go vet ./...
	@echo "All quality checks passed!"

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run security checks
security:
	@echo "Installing security tools..."
	@go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@echo "Running gosec..."
	gosec ./...
	@echo "Running govulncheck..."
	govulncheck ./...

# Run all checks (CI-like)
ci: deps test quality security
	@echo "All CI checks passed!"

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary for Linux AMD64"
	@echo "  dev           - Build the binary for current OS/arch"
	@echo "  install       - Install the binary to /usr/local/bin"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Download and tidy dependencies"
	@echo "  fmt           - Format code"
	@echo "  check-fmt     - Check code formatting"
	@echo "  lint          - Run linter"
	@echo "  quality       - Run all quality checks"
	@echo "  security      - Run security checks"
	@echo "  ci            - Run all checks (like CI)"
	@echo "  help          - Show this help message"

.PHONY: all build dev install test test-coverage clean deps fmt check-fmt lint quality security ci help
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
BUILD_DATE=$(shell date +%Y-%m-%d 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X coderaft/internal/commands.Version=$(VERSION) -X coderaft/internal/commands.CommitHash=$(GIT_COMMIT)"

# Binary name (auto-detect Windows)
BINARY_NAME=coderaft
BINARY_PATH=./cmd/coderaft

# Detect OS for platform-specific commands
UNAME_S := $(shell uname -s 2>/dev/null || echo Windows)

ifeq ($(UNAME_S),Windows_NT)
  BINARY_EXT=.exe
  RM_CMD=if exist $(BUILD_DIR) rmdir /s /q $(BUILD_DIR)
  MKDIR_CMD=if not exist $(BUILD_DIR) mkdir $(BUILD_DIR)
else ifeq ($(OS),Windows_NT)
  BINARY_EXT=.exe
  RM_CMD=if exist $(BUILD_DIR) rmdir /s /q $(BUILD_DIR)
  MKDIR_CMD=if not exist $(BUILD_DIR) mkdir $(BUILD_DIR)
else
  BINARY_EXT=
  RM_CMD=rm -rf $(BUILD_DIR)
  MKDIR_CMD=mkdir -p $(BUILD_DIR)
endif

# Build directory
BUILD_DIR=./build

# Default target
all: clean deps build

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build the binary for current platform
build:
	@echo "Building $(BINARY_NAME) version $(VERSION) (commit $(GIT_COMMIT))..."
	@$(MKDIR_CMD)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT) $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT)"

# Build for Linux AMD64
build-linux:
	@echo "Building $(BINARY_NAME) for Linux AMD64..."
	@$(MKDIR_CMD)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

# Build for Linux ARM64
build-linux-arm64:
	@echo "Building $(BINARY_NAME) for Linux ARM64..."
	@$(MKDIR_CMD)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64"

# Build for macOS AMD64
build-darwin:
	@echo "Building $(BINARY_NAME) for macOS AMD64..."
	@$(MKDIR_CMD)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64"

# Build for macOS ARM64 (Apple Silicon)
build-darwin-arm64:
	@echo "Building $(BINARY_NAME) for macOS ARM64 (Apple Silicon)..."
	@$(MKDIR_CMD)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64"

# Build for Windows AMD64
build-windows:
	@echo "Building $(BINARY_NAME) for Windows AMD64..."
	@$(MKDIR_CMD)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

# Build for Windows ARM64
build-windows-arm64:
	@echo "Building $(BINARY_NAME) for Windows ARM64..."
	@$(MKDIR_CMD)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe"

# Build for all platforms
build-all: build-linux build-linux-arm64 build-darwin build-darwin-arm64 build-windows build-windows-arm64
	@echo "All platform builds complete."

# Install the binary (Linux/macOS)
install: build
	@echo "Installing $(BINARY_NAME)..."
	@if [ "$$(uname)" = "Darwin" ] || [ "$$(uname)" = "Linux" ]; then \
		sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME); \
		sudo chmod +x /usr/local/bin/$(BINARY_NAME); \
		echo "Installed to /usr/local/bin/$(BINARY_NAME)"; \
	else \
		echo "On Windows, use: .\\build.ps1 -Install  or copy $(BUILD_DIR)/$(BINARY_NAME).exe to a directory in your PATH"; \
	fi

# Build for development (current OS/arch)
dev:
	@echo "Building $(BINARY_NAME) for development..."
	@$(MKDIR_CMD)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT) $(BINARY_PATH)
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME)$(BINARY_EXT)"

# Run tests
test:
	$(GOTEST) -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	@$(RM_CMD)

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
	@echo "  build              - Build binary for current platform"
	@echo "  build-linux        - Build for Linux AMD64"
	@echo "  build-linux-arm64  - Build for Linux ARM64"
	@echo "  build-darwin       - Build for macOS AMD64"
	@echo "  build-darwin-arm64 - Build for macOS ARM64 (Apple Silicon)"
	@echo "  build-windows      - Build for Windows AMD64"
	@echo "  build-windows-arm64- Build for Windows ARM64"
	@echo "  build-all          - Build for all platforms"
	@echo "  dev                - Build for current OS/arch (development)"
	@echo "  install            - Install binary to /usr/local/bin (Linux/macOS)"
	@echo "  test               - Run tests"
	@echo "  test-coverage      - Run tests with coverage report"
	@echo "  clean              - Clean build artifacts"
	@echo "  deps               - Download and tidy dependencies"
	@echo "  fmt                - Format code"
	@echo "  check-fmt          - Check code formatting"
	@echo "  lint               - Run linter"
	@echo "  quality            - Run all quality checks"
	@echo "  security           - Run security checks"
	@echo "  ci                 - Run all checks (like CI)"
	@echo "  help               - Show this help message"

.PHONY: all build build-linux build-linux-arm64 build-darwin build-darwin-arm64 build-windows build-windows-arm64 build-all dev install test test-coverage clean deps fmt check-fmt lint quality security ci help
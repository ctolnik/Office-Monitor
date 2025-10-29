.PHONY: build build-windows build-service clean test

# Build variables
BINARY_NAME=employee-agent.exe
VERSION?=1.0.0
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)

# Build Windows executable (from macOS/Linux)
build-windows:
	@echo "Building Windows agent..."
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "Built: $(BINARY_NAME)"

# Build Windows executable without console window (Windows service mode)
build-service:
	@echo "Building Windows agent (hidden mode)..."
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS) -H=windowsgui" -o $(BINARY_NAME) .
	@echo "Built: $(BINARY_NAME) (GUI-less)"

# Build for local testing (current OS)
build:
	@echo "Building for local platform..."
	go build -ldflags="$(LDFLAGS)" -o agent .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME) agent

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Build and package for distribution
package: build-service
	@echo "Creating package..."
	@mkdir -p dist
	@cp $(BINARY_NAME) dist/
	@cp config.yaml dist/config.yaml.example
	@echo "Package created in dist/"

.DEFAULT_GOAL := build-service

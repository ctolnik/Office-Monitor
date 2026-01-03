.PHONY: build build-windows build-service build-helper clean test

BINARY_NAME=agent-svc.exe
HELPER_NAME=agent-sh.exe
VERSION?=1.0.0
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)

build-windows:
	@echo "Building Windows agent..."
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "Built: $(BINARY_NAME)"

build-service:
	@echo "Building Windows agent (hidden mode)..."
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS) -H=windowsgui" -o $(BINARY_NAME) .
	@echo "Built: $(BINARY_NAME) (GUI-less)"

build-helper:
	@echo "Building session helper..."
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS) -H=windowsgui" -o $(HELPER_NAME) ./cmd/session-helper
	@echo "Built: $(HELPER_NAME)"

build-all: build-service build-helper
	@echo "All binaries built"

build:
	@echo "Building for local platform..."
	go build -ldflags="$(LDFLAGS)" -o agent .

clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME) $(HELPER_NAME) agent

test:
	@echo "Running tests..."
	go test -v ./...

deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

fmt:
	@echo "Formatting code..."
	go fmt ./...

lint:
	@echo "Running linter..."
	golangci-lint run ./...

package: build-all
	@echo "Creating package..."
	@mkdir -p dist
	@cp $(BINARY_NAME) dist/
	@cp $(HELPER_NAME) dist/
	@cp config.yaml dist/config.yaml.example
	@echo "Package created in dist/"

.DEFAULT_GOAL := build-all

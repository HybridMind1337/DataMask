# Makefile for DataMask

BINARY_NAME=datamask
VERSION=0.1.0
BUILD_DIR=bin
MODULE=github.com/HybridMind1337/DataMask

.PHONY: all build test vet clean docker build-all

all: vet test build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -trimpath -ldflags="-s -w -X $(MODULE)/internal/cli.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/datamask

test:
	@echo "Running tests..."
	go test -v -race ./...

vet:
	@echo "Running static analysis (go vet)..."
	go vet ./...

clean:
	@echo "Cleaning artifacts..."
	rm -rf $(BUILD_DIR)

docker:
	@echo "Building Docker container..."
	docker build -t hybridmind1337/datamask:latest -t hybridmind1337/datamask:$(VERSION) .

build-all:
	@echo "Cross-compiling multi-architecture releases..."
	mkdir -p $(BUILD_DIR)
	# Linux amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/datamask
	# Linux arm64
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/datamask
	# Darwin amd64
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/datamask
	# Darwin arm64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/datamask
	# Windows amd64
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/datamask
	@echo "Multi-arch binaries built in $(BUILD_DIR)/"

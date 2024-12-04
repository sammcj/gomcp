# Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOVET=$(GOCMD) vet
GOMOD=$(GOCMD) mod
BINARY_NAME=gomcp

# Build information
VERSION=$(shell git describe --tags --always --dirty)
COMMIT=$(shell git rev-parse --short HEAD)
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)

# Installation directory
PREFIX?=/usr/local
DESTDIR?=

# Determine OS and architecture
OS=$(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(shell uname -m)
ifeq ($(ARCH),x86_64)
    ARCH=amd64
endif

# Build directories
BUILD_DIR=bin
DIST_DIR=dist

# All source files
SRC=$(shell find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: all build test clean install uninstall lint vet mod-tidy help dist

all: lint test build

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

$(DIST_DIR):
	mkdir -p $(DIST_DIR)

# Build the application
build: $(BUILD_DIR)
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/gomcp

# Run tests with coverage
test:
	$(GOTEST) -v -race -cover ./...

example:
	go run ./example/create-example-db.go

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	go clean -testcache

# Install the application
install: build
	install -d $(DESTDIR)$(PREFIX)/bin
	install -m 755 $(BUILD_DIR)/$(BINARY_NAME) $(DESTDIR)$(PREFIX)/bin/

# Uninstall the application
uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/$(BINARY_NAME)

# Run linting
lint:
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

# Run go vet
vet:
	$(GOVET) ./...

# Update and tidy dependencies
mod-tidy:
	$(GOMOD) tidy
	$(GOMOD) verify

# Create distribution archives
dist: build $(DIST_DIR)
	tar -czf $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-$(OS)-$(ARCH).tar.gz -C $(BUILD_DIR) $(BINARY_NAME)
	cd $(BUILD_DIR) && sha256sum $(BINARY_NAME) > ../$(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-$(OS)-$(ARCH).sha256

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Build the application"
	@echo "  test        - Run tests with coverage"
	@echo "  clean       - Clean build artifacts"
	@echo "  install     - Install the application to $(PREFIX)/bin"
	@echo "  uninstall   - Remove the application from $(PREFIX)/bin"
	@echo "  lint        - Run linting checks"
	@echo "  vet        - Run go vet"
	@echo "  mod-tidy   - Update and verify dependencies"
	@echo "  dist       - Create distribution archives"
	@echo "  help       - Show this help message"

# Default target
.DEFAULT_GOAL := build

# kubectl-pprof Makefile

# 变量定义
APP_NAME := kubectl-pprof
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go 相关变量
GO := go
GOFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)

# 目录
BIN_DIR := bin
DIST_DIR := dist
CMD_DIR := cmd
PKG_DIR := pkg
INTERNAL_DIR := internal

# 默认目标
.PHONY: all
all: build

# 构建
.PHONY: build
build: clean
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$(APP_NAME) ./$(CMD_DIR)

# 交叉编译
.PHONY: build-all
build-all: clean
	@echo "Cross-compiling $(APP_NAME)..."
	@mkdir -p $(DIST_DIR)
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 ./$(CMD_DIR)
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 ./$(CMD_DIR)
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 ./$(CMD_DIR)
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 ./$(CMD_DIR)
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe ./$(CMD_DIR)

# 安装
.PHONY: install
install: build
	@echo "Installing $(APP_NAME)..."
	cp $(BIN_DIR)/$(APP_NAME) /usr/local/bin/

# 测试
.PHONY: test
test:
	@echo "Running tests..."
	$(GO) test -v ./...

# 测试覆盖率
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 基准测试
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

# 代码检查
.PHONY: lint
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found, installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

# 格式化代码
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# 整理依赖
.PHONY: tidy
tidy:
	@echo "Tidying dependencies..."
	$(GO) mod tidy

# 下载依赖
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download

# 清理
.PHONY: clean
clean:
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR) $(DIST_DIR) coverage.out coverage.html

# 生成文档
.PHONY: docs
docs:
	@echo "Generating documentation..."
	@mkdir -p docs/api
	$(GO) run ./$(CMD_DIR) --help > docs/usage.txt

# 运行示例
.PHONY: example
example: build
	@echo "Running example..."
	./$(BIN_DIR)/$(APP_NAME) --help

# 开发模式
.PHONY: dev
dev:
	@echo "Starting development mode..."
	@which air > /dev/null || (echo "air not found, installing..." && go install github.com/cosmtrek/air@latest)
	air

# 检查依赖更新
.PHONY: check-updates
check-updates:
	@echo "Checking for dependency updates..."
	@which go-mod-outdated > /dev/null || (echo "go-mod-outdated not found, installing..." && go install github.com/psampaz/go-mod-outdated@latest)
	$(GO) list -u -m -json all | go-mod-outdated -update -direct

# 安全检查
.PHONY: security
security:
	@echo "Running security checks..."
	@which gosec > /dev/null || (echo "gosec not found, installing..." && go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest)
	gosec ./...

# 生成版本信息
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"
	@echo "Go version: $(shell go version)"
	@echo "Platform: $(GOOS)/$(GOARCH)"

# Docker 构建
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) .
	docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest

# 发布准备
.PHONY: release
release: clean test lint build-all
	@echo "Preparing release $(VERSION)..."
	@mkdir -p $(DIST_DIR)/checksums
	# 生成校验和
	cd $(DIST_DIR) && sha256sum * > checksums/sha256sums.txt
	@echo "Release $(VERSION) ready in $(DIST_DIR)/"

# 帮助
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  build-all     - Cross-compile for all platforms"
	@echo "  install       - Install the application"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  bench         - Run benchmarks"
	@echo "  lint          - Run linters"
	@echo "  fmt           - Format code"
	@echo "  tidy          - Tidy dependencies"
	@echo "  deps          - Download dependencies"
	@echo "  clean         - Clean build artifacts"
	@echo "  docs          - Generate documentation"
	@echo "  example       - Run example"
	@echo "  dev           - Start development mode"
	@echo "  check-updates - Check for dependency updates"
	@echo "  security      - Run security checks"
	@echo "  version       - Show version information"
	@echo "  docker-build  - Build Docker image"
	@echo "  release       - Prepare release"
	@echo "  help          - Show this help"
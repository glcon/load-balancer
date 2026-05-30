.PHONY: help clean run build test lint benchmark infra-up infra-down

# Configuration
APP_NAME := lb
BIN_DIR := bin

# ==============================================================================

## help: Print this help message
help:
	@echo "Usage:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## clean: Wipe out old binaries
clean:
	@echo "Cleaning up..."
	@rm -rf $(BIN_DIR)
	@echo "Clean complete."

# ==============================================================================

## run: Run the load balancer
run:
	@echo "Running with race detector..."
	@go run ./cmd/lb/main.go

## build: Build the load balancer binary
build: clean
	@echo "Building binary..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/lb/main.go
	@echo "Build complete: $(BIN_DIR)/$(APP_NAME)"

# ==============================================================================

## test: Run unit tests with race detector enabled
test:
	@echo "Running tests..."
	@go test -race -v ./...

# ==============================================================================

## infra-up: Spin up Docker infrastructure
infra-up:
	@echo "Starting infrastructure..."
	@cd deploy && docker compose up -d

## infra-down: Tear down Docker infrastructure
infra-down:
	@echo "Tearing down infrastructure..."
	@cd deploy && docker compose down

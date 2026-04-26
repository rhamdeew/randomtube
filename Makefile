.PHONY: build run run-dev clean test test-coverage migrate deps help

.DEFAULT_GOAL := help

build: ## Build the binary
	go build -o randomtube .

help: ## Show this help message
	@echo ""

	@echo "██████╗░░█████╗░███╗░░██╗██████╗░░█████╗░███╗░░░███╗████████╗██╗░░░██╗██████╗░███████╗"
	@echo "██╔══██╗██╔══██╗████╗░██║██╔══██╗██╔══██╗████╗░████║╚══██╔══╝██║░░░██║██╔══██╗██╔════╝"
	@echo "██████╔╝███████║██╔██╗██║██║░░██║██║░░██║██╔████╔██║░░░██║░░░██║░░░██║██████╦╝█████╗░░"
	@echo "██╔══██╗██╔══██║██║╚████║██║░░██║██║░░██║██║╚██╔╝██║░░░██║░░░██║░░░██║██╔══██╗██╔══╝░░"
	@echo "██║░░██║██║░░██║██║░╚███║██████╔╝╚█████╔╝██║░╚═╝░██║░░░██║░░░╚██████╔╝██████╦╝███████╗"
	@echo "╚═╝░░╚═╝╚═╝░░╚═╝╚═╝░░╚══╝╚═════╝░░╚════╝░╚═╝░░░░░╚═╝░░░╚═╝░░░░╚═════╝░╚═════╝░╚══════╝"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: build ## Build and run (requires ADMIN_PASSWORD env var)
	./randomtube

run-dev: build ## Build and run with dev defaults
	ADMIN_PASSWORD=$${ADMIN_PASSWORD:-dev} \
	SESSION_SECRET=$${SESSION_SECRET:-dev-secret} \
	./randomtube

clean: ## Remove build artifacts
	rm -f randomtube

deps: ## Download Go module dependencies
	go mod download

test: ## Run all tests
	go test -v ./...

test-coverage: ## Run tests with coverage report (opens in browser)
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

migrate: ## Import MySQL dump into SQLite (DUMP=/path/to/dump.sql DB=randomtube.db)
	go run ./migrate/ -dump $${DUMP:-randomtube_dump.sql} -db $${DB:-randomtube.db}

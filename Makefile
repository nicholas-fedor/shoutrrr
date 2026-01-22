# Shoutrrr Makefile
# Comprehensive build, test, and deployment targets for the Shoutrrr project

# Variables
BINARY_NAME=shoutrrr
GO=go
DOCKER=docker
GORELEASER=goreleaser
GOLANGCI_LINT=golangci-lint

# Default target
help: ## Show this help message
	@echo "Shoutrrr Project Makefile"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Parameterized test targets:"
	@echo "  test-unit                Run unit tests for a specific service (usage: make test-unit <service>)"
	@echo "  test-integration         Run integration tests for a specific service (usage: make test-integration <service>)"
	@echo "  test-e2e                 Run e2e tests for a specific service (usage: make test-e2e <service>)"

# =============================================================================
# Development Targets
# =============================================================================

.PHONY: build test test-unit test-integration test-e2e lint vet run setup install

build: ## Build the application binary
	bash ./scripts/build.sh

test: ## Run unit tests only
	$(GO) test -timeout 30s -v -coverprofile coverage.out -covermode atomic $(shell go list ./... | grep -v testing/...)

test-unit: ## Run unit tests for a specific service (usage: make test-unit <service>)
	@SVC=`echo $(MAKECMDGOALS) | cut -d' ' -f2`; \
	if [ -z "$$SVC" ]; then echo "Usage: make test-unit <service>"; exit 1; fi; \
	SERVICE_DIR=`find ./pkg/services -name "$$SVC" -type d`; \
	$(GO) test -timeout 30s -v $$SERVICE_DIR

test-integration: ## Run integration tests for a specific service (usage: make test-integration <service>)
	@SVC=`echo $(MAKECMDGOALS) | cut -d' ' -f2`; \
	if [ -z "$$SVC" ]; then echo "Usage: make test-integration <service>"; exit 1; fi; \
	$(GO) test -timeout 30s -v ./testing/integration/$$SVC

test-e2e: ## Run e2e tests for a specific service (usage: make test-e2e <service>)
	@SVC=`echo $(MAKECMDGOALS) | cut -d' ' -f2`; \
	if [ -z "$$SVC" ]; then echo "Usage: make test-e2e <service>"; exit 1; fi; \
	$(GO) test -timeout 30s -v ./testing/e2e/$$SVC

lint: ## Run linter and fix issues
	$(GOLANGCI_LINT) run --fix --config build/golangci-lint/golangci.yaml ./...

vet: ## Run Go vet
	$(GO) vet ./...

fmt: ## Run formatter
	$(GOLANGCI_LINT) fmt --config build/golangci-lint/golangci.yaml ./...

run: ## Run the application
	$(GO) run ./...

install: ## Install the application
	$(GO) install ./...

# =============================================================================
# Dependency Management
# =============================================================================

.PHONY: mod-tidy mod-download

mod-tidy: ## Tidy and clean up Go modules
	$(GO) mod tidy

mod-download: ## Download Go module dependencies
	$(GO) mod download

# =============================================================================
# Documentation Targets
# =============================================================================

.PHONY: docs docs-setup docs-build docs-serve docs-activate docs-deactivate

docs: docs-setup docs-serve ## Build and serve documentation site for local development

docs-setup: ## Create virtual environment and install Mkdocs dependencies
	python3 -m venv shoutrrr-docs && chmod +x shoutrrr-docs/bin/activate && . shoutrrr-docs/bin/activate && pip install -r build/mkdocs/docs-requirements.txt

docs-gen: ## Generate service configuration documentation
	bash ./scripts/generate-service-config-docs.sh

docs-build: docs-gen ## Build Mkdocs documentation site
	. shoutrrr-docs/bin/activate && mkdocs build --config-file build/mkdocs/mkdocs.yaml

docs-serve: ## Serve Mkdocs documentation site locally
	. shoutrrr-docs/bin/activate && mkdocs serve --config-file build/mkdocs/mkdocs.yaml --livereload

docs-activate: ## Activate the virtual environment for documentation
	@echo "Run '. shoutrrr-docs/bin/activate' to activate the virtual environment."

docs-deactivate: ## Show instructions to deactivate the virtual environment
	@echo "Run 'deactivate' to exit the virtual environment."

# =============================================================================
# Release Targets
# =============================================================================

.PHONY: release

release: ## Create a new release using GoReleaser
	$(GORELEASER) release --clean

# =============================================================================
# Docker Targets
# =============================================================================

.PHONY: docker-build docker-run docker-push

docker-build: ## Build Docker image
	$(DOCKER) build -t $(BINARY_NAME) .

docker-run: ## Run Docker container
	$(DOCKER) run -p 8080:8080 $(BINARY_NAME)

docker-push: ## Push Docker image (requires proper tagging)
	$(DOCKER) push $(BINARY_NAME)

# =============================================================================
# Utility Targets
# =============================================================================

.PHONY: clean

clean: ## Clean build artifacts
	rm -rf bin/

.PHONY: %
%:
	@:

# =============================================================================
# Makefile for Terraform Provider Trueform
# =============================================================================

HOSTNAME=registry.terraform.io
NAMESPACE=trueform
NAME=trueform
BINARY=terraform-provider-${NAME}
VERSION?=0.1.0
OS_ARCH=$(shell go env GOOS)_$(shell go env GOARCH)

default: build

# Build the provider
build:
	go build -o ${BINARY}

# Install the provider locally for development
install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	cp ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

# Run unit tests
test:
	go test -v ./...

# Run acceptance tests (requires TrueNAS instance)
testacc:
	TF_ACC=1 go test -v ./... -timeout 120m

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	terraform fmt -recursive test-resources/

# Generate documentation
docs:
	go generate ./...

# Clean build artifacts
clean:
	rm -f ${BINARY}
	rm -rf dist/

# Test release locally (no publish)
release-dry-run:
	goreleaser release --snapshot --clean

# Verify goreleaser configuration
release-check:
	goreleaser check

# Create dev_overrides configuration
dev-override:
	@echo 'provider_installation {' > ~/.terraformrc
	@echo '  dev_overrides {' >> ~/.terraformrc
	@echo '    "${HOSTNAME}/${NAMESPACE}/${NAME}" = "$(shell pwd)"' >> ~/.terraformrc
	@echo '  }' >> ~/.terraformrc
	@echo '  direct {}' >> ~/.terraformrc
	@echo '}' >> ~/.terraformrc
	@echo "Created ~/.terraformrc with dev_overrides pointing to $(shell pwd)"

# Show help
help:
	@echo "Available targets:"
	@echo "  build          - Build the provider binary"
	@echo "  install        - Install provider to local plugins directory"
	@echo "  test           - Run unit tests"
	@echo "  testacc        - Run acceptance tests (requires TrueNAS)"
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format Go and Terraform code"
	@echo "  docs           - Generate documentation"
	@echo "  clean          - Remove build artifacts"
	@echo "  release-dry-run - Test release process locally"
	@echo "  release-check  - Verify goreleaser configuration"
	@echo "  dev-override   - Create ~/.terraformrc with dev_overrides"
	@echo "  help           - Show this help message"

.PHONY: default build install test testacc lint fmt docs clean release-dry-run release-check dev-override help

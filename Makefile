.PHONY: build test run clean docker-build docker-push deploy-dynamic cleanup-dynamic

# Build the LEAD framework with dynamic network topology
build:
	go build -o lead-scheduler ./cmd/scheduler

# Run tests
test:
	go test -v ./tests/...

# Run benchmarks
benchmark:
	go test -bench=. ./tests/...

# Run the LEAD scheduler
run:
	go run ./cmd/scheduler

# Run the LEAD scheduler with dynamic network monitoring
run-dynamic:
	go run ./cmd/scheduler --enable-dynamic-network-monitoring --enable-geographic-awareness

# Clean build artifacts
clean:
	rm -f lead-scheduler
	rm -rf k8s-manifests
	rm -rf hotel-k8s-manifests

# Docker operations
docker-build:
	docker build -t lead-scheduler:latest .

docker-push:
	docker push lead-scheduler:latest

# Deploy dynamic network topology
deploy-dynamic:
	./scripts/deploy-dynamic-network-topology.sh

# Cleanup dynamic network topology
cleanup-dynamic:
	./scripts/cleanup-dynamic-network-topology.sh

# Format code
fmt:
	go fmt ./...

# Check for linting issues
lint:
	golangci-lint run

# Install dependencies (if using external deps)
deps:
	go mod tidy

# Show help
help:
	@echo "Available targets:"
	@echo "  build         - Build the LEAD scheduler binary"
	@echo "  test          - Run all tests"
	@echo "  benchmark     - Run benchmarks"
	@echo "  run           - Run the LEAD scheduler"
	@echo "  run-dynamic   - Run with dynamic network monitoring"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format Go code"
	@echo "  lint          - Run linter (requires golangci-lint)"
	@echo "  deps          - Install dependencies"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-push   - Push Docker image"
	@echo "  deploy-dynamic - Deploy with dynamic network topology"
	@echo "  cleanup-dynamic - Cleanup dynamic network topology deployment"
	@echo "  help          - Show this help message"

.PHONY: build test run clean

# Build the LEAD framework
build:
	go build -o lead-framework main.go

# Run tests
test:
	go test -v ./tests/...

# Run benchmarks
benchmark:
	go test -bench=. ./tests/...

# Run the example
run:
	go run main.go

# Run the hotel reservation example
run-example:
	go run examples/hotel_reservation.go

# Clean build artifacts
clean:
	rm -f lead-framework
	rm -rf k8s-manifests
	rm -rf hotel-k8s-manifests

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
	@echo "  build       - Build the LEAD framework binary"
	@echo "  test        - Run all tests"
	@echo "  benchmark   - Run benchmarks"
	@echo "  run         - Run the basic example"
	@echo "  run-example - Run the hotel reservation example"
	@echo "  clean       - Clean build artifacts"
	@echo "  fmt         - Format Go code"
	@echo "  lint        - Run linter (requires golangci-lint)"
	@echo "  deps        - Install dependencies"
	@echo "  help        - Show this help message"

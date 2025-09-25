
# Multi-stage build for LEAD Scheduler with Dynamic Network Topology
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install git, ca-certificates, and curl for health checks
RUN apk add --no-cache git ca-certificates tzdata curl

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the scheduler binary with optimizations for dynamic network monitoring
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty)" \
    -o lead-scheduler ./cmd/scheduler

# Final stage
FROM alpine:latest

# Install ca-certificates, curl for health checks, and timezone data
RUN apk --no-cache add ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1001 -S lead && \
    adduser -u 1001 -S lead -G lead

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/lead-scheduler .

# Copy configuration templates and documentation
COPY --from=builder /app/MANUAL_CLUSTER_SETUP.md ./docs/
COPY --from=builder /app/PROMETHEUS_CONFIGURATION.md ./docs/

# Create directories for output and logs
RUN mkdir -p /app/k8s-manifests /app/config /app/logs && \
    chown -R lead:lead /app

# Switch to non-root user
USER lead

# Expose port for metrics and health checks
EXPOSE 10259

# Health check with improved timeout for dynamic network monitoring
HEALTHCHECK --interval=30s --timeout=15s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:10259/healthz || exit 1

# Set environment variables for dynamic network topology
ENV LEAD_NETWORK_MONITORING=true \
    LEAD_DYNAMIC_DISCOVERY=true \
    LEAD_PROMETHEUS_INTEGRATION=true \
    LEAD_GEOGRAPHIC_AWARENESS=true

# Run the scheduler with dynamic network topology support
CMD ["./lead-scheduler", "--enable-dynamic-network-monitoring", "--enable-geographic-awareness"]


ت# Multi-stage build for LEAD Scheduler
FROM golang:1.21-alpine AS builder

# Set working directory
WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the scheduler binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o lead-scheduler ./cmd/scheduler

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S lead && \
    adduser -u 1001 -S lead -G lead

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/lead-scheduler .

# Copy example configurations
COPY --from=builder /app/examples ./examples

# Create directories for output
RUN mkdir -p /app/k8s-manifests /app/config && \
    chown -R lead:lead /app

# Switch to non-root user
USER lead

# Expose port for metrics
EXPOSE 10259

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:10259/healthz || exit 1

# Run the scheduler
CMD ["./lead-scheduler"]

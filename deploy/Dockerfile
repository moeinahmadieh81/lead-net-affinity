# =========================
# Stage 1: Build
# =========================
FROM golang:1.24 AS builder

# Static linux/amd64 build
ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest
COPY . .

# Build binary
RUN go build -o /lead-net-affinity ./cmd/lead-net-affinity

# =========================
# Stage 2: Runtime
# =========================
# Use a small base with certs so HTTPS Prometheus works too.
FROM alpine:3.20

# Install CA certs (for HTTPS Prometheus, if ever needed)
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -S app && adduser -S app -G app

WORKDIR /

COPY --from=builder /lead-net-affinity /lead-net-affinity

USER app:app

ENTRYPOINT ["/lead-net-affinity"]

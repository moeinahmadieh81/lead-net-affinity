# =========================
# Stage 1: Build
# =========================
FROM --platform=linux/amd64 golang:1.24 AS builder

# Enable Go modules and stricter builds
ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

# Pre-copy go.mod/go.sum to cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build static binary
RUN go build -o /lead-net-affinity ./cmd/lead-net-affinity

# =========================
# Stage 2: Runtime
# =========================
FROM --platform=linux/amd64 gcr.io/distroless/static:nonroot

# Where config.yaml will be mounted (matches LEAD_NET_CONFIG default)
ENV LEAD_NET_CONFIG=/etc/lead-net-affinity/config.yaml

WORKDIR /

# Copy binary from builder
COPY --from=builder /lead-net-affinity /lead-net-affinity

USER nonroot:nonroot

ENTRYPOINT ["/lead-net-affinity"]

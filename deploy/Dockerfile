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
# If Prometheus is HTTP, static is fine.
# If Prometheus is HTTPS, better to use base:nonroot for CA certs.
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /lead-net-affinity /lead-net-affinity

USER nonroot:nonroot

ENTRYPOINT ["/lead-net-affinity"]

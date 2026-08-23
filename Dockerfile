# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Download dependencies first so Docker can cache this layer.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source and build a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o annave-pdf-engine \
    ./cmd/server

# ── Final stage ──────────────────────────────────────────────────────────────
# scratch has no shell, no libc, no package manager.
# The binary must be fully static (CGO_ENABLED=0 ensures this).
FROM scratch

COPY --from=builder /build/annave-pdf-engine /annave-pdf-engine

# Railway (and other PaaS) set PORT automatically. Default is 5741 for local dev.
EXPOSE 5741

ENTRYPOINT ["/annave-pdf-engine"]

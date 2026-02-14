# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE=api
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /bin/service ./cmd/${SERVICE}

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget
RUN adduser -D -u 1000 appuser

WORKDIR /app
COPY --from=builder /bin/service /usr/local/bin/service
COPY config.yaml /app/config.yaml
COPY manifest.json /app/manifest.json
COPY api/ /app/api/

LABEL org.opencontainers.image.source="https://github.com/bilbopass/bilbopass"
LABEL org.opencontainers.image.description="BilboPass transit system"

USER appuser

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/v1/health || exit 1

ENTRYPOINT ["/usr/local/bin/service"]

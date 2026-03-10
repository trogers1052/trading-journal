# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /trading-journal ./cmd/journal
RUN CGO_ENABLED=0 GOOS=linux go build -o /healthcheck ./cmd/healthcheck

# Run stage
FROM gcr.io/distroless/static-debian12

COPY --from=builder /trading-journal /trading-journal
COPY --from=builder /healthcheck /healthcheck

USER nonroot:nonroot

HEALTHCHECK --interval=30s --timeout=5s --retries=3 --start-period=15s \
    CMD ["/healthcheck"]

ENTRYPOINT ["/trading-journal"]

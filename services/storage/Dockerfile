# Build from repo root: docker build -f services/storage/Dockerfile .
FROM golang:1-alpine AS builder

WORKDIR /build

# Copy proto module (needed for replace directive in go.mod)
COPY proto/go.mod proto/go.sum ./proto/
COPY proto/gen/ ./proto/gen/

# Copy and download deps
COPY services/storage/go.mod services/storage/go.sum ./services/storage/
WORKDIR /build/services/storage
RUN go mod download

# Copy source and build
WORKDIR /build
COPY services/storage/ ./services/storage/
WORKDIR /build/services/storage
RUN CGO_ENABLED=0 go build -o /garance-storage .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /garance-storage /usr/local/bin/garance-storage
COPY --from=builder /build/services/storage/migrations/ /app/migrations/

WORKDIR /app
ENV LISTEN_ADDR=0.0.0.0:4002
EXPOSE 4002
CMD ["garance-storage"]

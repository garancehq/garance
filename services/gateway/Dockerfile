# Build from repo root: docker build -f services/gateway/Dockerfile .
FROM golang:1-alpine AS builder

WORKDIR /build

# Copy proto module (needed for replace directive in go.mod)
COPY proto/go.mod proto/go.sum ./proto/
COPY proto/gen/ ./proto/gen/

# Copy and download deps
COPY services/gateway/go.mod services/gateway/go.sum ./services/gateway/
WORKDIR /build/services/gateway
RUN go mod download

# Copy source and build
WORKDIR /build
COPY services/gateway/ ./services/gateway/
WORKDIR /build/services/gateway
RUN CGO_ENABLED=0 go build -o /garance-gateway .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /garance-gateway /usr/local/bin/garance-gateway

ENV LISTEN_ADDR=0.0.0.0:8080
EXPOSE 8080
CMD ["garance-gateway"]

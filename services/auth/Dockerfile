# Build from repo root: docker build -f services/auth/Dockerfile .
FROM golang:1-alpine AS builder

WORKDIR /build

# Copy proto module (needed for replace directive in go.mod)
COPY proto/go.mod proto/go.sum ./proto/
COPY proto/gen/ ./proto/gen/

# Copy and download deps
COPY services/auth/go.mod services/auth/go.sum ./services/auth/
WORKDIR /build/services/auth
RUN go mod download

# Copy source and build
WORKDIR /build
COPY services/auth/ ./services/auth/
WORKDIR /build/services/auth
RUN CGO_ENABLED=0 go build -o /garance-auth .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /garance-auth /usr/local/bin/garance-auth
COPY --from=builder /build/services/auth/migrations/ /app/migrations/

WORKDIR /app
ENV LISTEN_ADDR=0.0.0.0:4001
EXPOSE 4001
CMD ["garance-auth"]

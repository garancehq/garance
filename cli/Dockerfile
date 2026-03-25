# cli/Dockerfile
FROM golang:1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-X github.com/garancehq/garance/cli/cmd.Version=0.1.0" -o /garance .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=builder /garance /usr/local/bin/garance
CMD ["garance"]

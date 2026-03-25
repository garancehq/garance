FROM golang:1-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /garance-gateway .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /garance-gateway /usr/local/bin/garance-gateway

ENV LISTEN_ADDR=0.0.0.0:8080
EXPOSE 8080

CMD ["garance-gateway"]

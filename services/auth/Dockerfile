FROM golang:1-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /garance-auth .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /garance-auth /usr/local/bin/garance-auth
COPY migrations/ /app/migrations/

WORKDIR /app
ENV LISTEN_ADDR=0.0.0.0:4001
EXPOSE 4001

CMD ["garance-auth"]

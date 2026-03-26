module github.com/garancehq/garance/services/gateway

go 1.25.4

require (
	github.com/garancehq/garance/proto v0.0.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	google.golang.org/grpc v1.79.3
)

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	nhooyr.io/websocket v1.8.17 // indirect
)

replace github.com/garancehq/garance/proto => ../../proto

module github.com/garancehq/garance/services/gateway

go 1.25.4

require (
	github.com/garancehq/garance/proto v0.0.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	google.golang.org/grpc v1.80.0
	nhooyr.io/websocket v1.8.17
)

require (
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260120221211-b8f7ae30c516 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/garancehq/garance/proto => ../../proto

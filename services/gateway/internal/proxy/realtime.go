package proxy

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"

	"nhooyr.io/websocket"
)

type RealtimeProxy struct {
	targetURL string
}

func NewRealtimeProxy() *RealtimeProxy {
	target := os.Getenv("REALTIME_WS_URL")
	if target == "" {
		target = "ws://realtime:4003"
	}
	return &RealtimeProxy{targetURL: target}
}

func (p *RealtimeProxy) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/realtime", p.ProxyWebSocket)
	mux.HandleFunc("/realtime/", p.ProxyWebSocket)
}

func (p *RealtimeProxy) ProxyWebSocket(w http.ResponseWriter, r *http.Request) {
	// Accept the client WebSocket connection
	clientConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("failed to accept websocket: %v", err)
		return
	}
	defer clientConn.CloseNow()

	// Connect to the upstream Realtime service
	upstreamURL := p.targetURL + "/socket/websocket"
	upstreamConn, _, err := websocket.Dial(r.Context(), upstreamURL, nil)
	if err != nil {
		log.Printf("failed to connect to realtime service: %v", err)
		clientConn.Close(websocket.StatusInternalError, "upstream unavailable")
		return
	}
	defer upstreamConn.CloseNow()

	// Bidirectional proxy
	errc := make(chan error, 2)

	// Client -> Upstream
	go func() {
		errc <- proxyWS(r.Context(), clientConn, upstreamConn)
	}()

	// Upstream -> Client
	go func() {
		errc <- proxyWS(r.Context(), upstreamConn, clientConn)
	}()

	// Wait for either direction to close
	<-errc
}

func proxyWS(ctx context.Context, src, dst *websocket.Conn) error {
	for {
		msgType, reader, err := src.Reader(ctx)
		if err != nil {
			return err
		}

		writer, err := dst.Writer(ctx, msgType)
		if err != nil {
			return err
		}

		if _, err := io.Copy(writer, reader); err != nil {
			return err
		}

		if err := writer.Close(); err != nil {
			return err
		}
	}
}

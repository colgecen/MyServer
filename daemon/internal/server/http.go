package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const (
	protocolVersion = "1.0"
	readLimit       = 1 << 20 // 1 MiB per WS message
	writeTimeout    = 10 * time.Second
	pingInterval    = 25 * time.Second
)

// HTTPHandler is the optional HTTP router used by tests / health endpoints.
type HTTPHandler struct {
	mux *http.ServeMux
	s   *HTTPServer
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// HTTPServer is the concrete Server implementation: stdlib net/http + coder/websocket.
type HTTPServer struct {
	cfg    Config
	httpSrv *http.Server
	wsPath string

	mu       sync.Mutex
	conns    map[*websocket.Conn]struct{}
	closed   atomic.Bool
	listener net.Listener

	onMessage func(ctx context.Context, c *websocket.Conn, msg []byte)
}

func New(cfg Config) *HTTPServer {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:4096"
	}
	return &HTTPServer{
		cfg:    cfg,
		wsPath: "/ws",
		conns:  make(map[*websocket.Conn]struct{}),
	}
}

// OnMessage registers the inbound WebSocket message handler.
func (s *HTTPServer) OnMessage(fn func(ctx context.Context, c *websocket.Conn, msg []byte)) {
	s.mu.Lock()
	s.onMessage = fn
	s.mu.Unlock()
}

// Handler returns the underlying http.Handler (useful for tests).
func (s *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/telemetry", s.handleTelemetry)
	mux.HandleFunc("/ws", s.handleWS)
	return mux
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r.RemoteAddr) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":           "ok",
		"version":          s.cfg.Version,
		"protocol_version": protocolVersion,
		"uptime_seconds":   uptimeSeconds(),
	})
}

func (s *HTTPServer) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r.RemoteAddr) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"gpu_util":     0,
		"vram_used_mb": 0,
		"vram_total_mb": 0,
		"cpu_percent":  0,
		"mem_percent":  0,
	})
}

func (s *HTTPServer) handleWS(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r.RemoteAddr) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if s.closed.Load() {
		http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // localhost only; UI uses ws://127.0.0.1
	})
	if err != nil {
		log.Printf("ws accept error: %v", err)
		return
	}
	c.SetReadLimit(readLimit)

	s.trackConn(c)
	defer s.untrackConn(c)

	// handshake ping
	hello := map[string]any{
		"type":             "hello",
		"protocol_version": protocolVersion,
		"version":          s.cfg.Version,
	}
	if err := writeJSON(c, hello); err != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go s.heartbeat(ctx, c)

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		s.mu.Lock()
		fn := s.onMessage
		s.mu.Unlock()
		if fn != nil {
			fn(ctx, c, data)
		}
	}
}

func (s *HTTPServer) heartbeat(ctx context.Context, c *websocket.Conn) {
	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			_ = c.Ping(wctx)
			cancel()
		}
	}
}

func (s *HTTPServer) trackConn(c *websocket.Conn) {
	s.mu.Lock()
	s.conns[c] = struct{}{}
	s.mu.Unlock()
}

func (s *HTTPServer) untrackConn(c *websocket.Conn) {
	s.mu.Lock()
	delete(s.conns, c)
	s.mu.Unlock()
}

// Broadcast sends an event payload to all connected clients.
func (s *HTTPServer) Broadcast(payload any) {
	s.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()
	for _, c := range conns {
		wctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		_ = writeJSONCtx(wctx, c, payload)
		cancel()
	}
}

func (s *HTTPServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Addr, err)
	}
	s.listener = ln
	s.httpSrv = &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listening on http://%s", ln.Addr())
	go func() {
		if err := s.httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server error: %v", err)
		}
	}()
	return nil
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.closed.Store(true)

	s.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.conns = make(map[*websocket.Conn]struct{})
	s.mu.Unlock()

	for _, c := range conns {
		cancelCtx, cancel := context.WithTimeout(ctx, writeTimeout)
		_ = c.Close(websocket.StatusGoingAway, "server shutdown")
		cancel()
		_ = cancelCtx
	}

	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

// Addr returns the actual bound address (useful when port 0 is used in tests).
func (s *HTTPServer) Addr() string {
	if s.listener == nil {
		return s.cfg.Addr
	}
	return s.listener.Addr().String()
}

func writeJSON(c *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	return c.Write(ctx, websocket.MessageText, data)
}

func writeJSONCtx(ctx context.Context, c *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.Write(ctx, websocket.MessageText, data)
}

// isLoopback rejects any non-loopback peer. Defense in depth: the daemon must
// refuse connections from non-localhost addresses even if the listen socket
// were ever misconfigured.
func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

var startTime = time.Now()

func uptimeSeconds() int64 { return int64(time.Since(startTime).Seconds()) }

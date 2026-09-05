package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/anomalyco/myserver/daemon/internal/app"
	"github.com/anomalyco/myserver/daemon/internal/protocol"
	"github.com/anomalyco/myserver/daemon/internal/server"
	"github.com/anomalyco/myserver/daemon/internal/store"
	"github.com/anomalyco/myserver/daemon/internal/telemetry"
)

const version = "1.14.33"

func main() {
	addr := flag.String("addr", envOr("MYSERVERD_ADDR", "127.0.0.1:4096"), "listen address")
	dataDir := flag.String("data", envOr("MYSERVERD_DATA", "./daemon/data"), "data directory for audit/workspace")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("myserverd v%s starting on %s (data: %s)", version, *addr, *dataDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("data dir: %v", err)
	}

	srv := server.New(server.Config{
		Addr:    *addr,
		Version: version,
	})

	// Initialize audit store
	auditPath := filepath.Join(*dataDir, "audit.db")
	auditStore, err := store.Open(auditPath)
	if err != nil {
		log.Fatalf("Failed to initialize audit store: %v", err)
	}
	defer auditStore.Close()

	application := app.New(srv, auditStore)

	// Start telemetry poller (broadcasts every 1s to all WS clients)
	collector := telemetry.NewCollector()
	poller := telemetry.NewPoller(collector, &serverBroadcaster{srv: srv})
	go poller.Start(ctx)

	if err := srv.Start(ctx); err != nil {
		log.Fatalf("server start failed: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("received %s, shutting down...", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		log.Printf("graceful shutdown error: %v", err)
	}
	application.Shutdown()
	log.Print("myserverd stopped")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// serverBroadcaster adapts server.HTTPServer.Broadcast to telemetry.Broadcaster.
type serverBroadcaster struct {
	srv *server.HTTPServer
}

func (b *serverBroadcaster) Broadcast(env *protocol.Envelope) error {
	b.srv.Broadcast(env)
	return nil
}

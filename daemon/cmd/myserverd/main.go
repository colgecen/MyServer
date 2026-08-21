package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anomalyco/myserver/daemon/internal/app"
	"github.com/anomalyco/myserver/daemon/internal/server"
)

const version = "1.14.33"

func main() {
	addr := flag.String("addr", envOr("MYSERVERD_ADDR", "127.0.0.1:4096"), "listen address")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("myserverd v%s starting on %s", version, *addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := server.New(server.Config{
		Addr:    *addr,
		Version: version,
	})
	application := app.New(srv)

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

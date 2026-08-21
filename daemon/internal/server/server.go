package server

import "context"

type Config struct {
	Addr    string
	Version string
}

type Server interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

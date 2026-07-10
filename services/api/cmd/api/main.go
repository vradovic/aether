package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/vradovic/aether/services/api/internal/config"
	"github.com/vradovic/aether/services/api/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config: ", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx := context.Background()

	s, err := server.NewServer(ctx, cfg, logger)
	if err != nil {
		log.Fatal("failed to create server: ", err)
	}
	defer s.Close()
	if err := s.Start(); err != nil {
		log.Fatal("failed to start server: ", err)
	}
}

package config

import (
	"fmt"
	"os"
)

type Config struct {
	ServerAddress string
	DbAddress     string
}

func Load() (*Config, error) {
	cfg := &Config{
		ServerAddress: os.Getenv("SERVER_ADDRESS"),
		DbAddress:     os.Getenv("DB_ADDRESS"),
	}

	if cfg.ServerAddress == "" {
		return nil, fmt.Errorf("SERVER_ADDRESS is required")
	}

	if cfg.DbAddress == "" {
		return nil, fmt.Errorf("DB_ADDRESS is required")
	}

	return cfg, nil
}

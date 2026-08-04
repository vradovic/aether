package api

import (
	"fmt"
	"os"
)

const minJWTSigningKeyLengthBytes = 32

type Config struct {
	ServerAddress  string
	DbAddress      string
	ScyllaHosts    string
	ScyllaKeyspace string
	JWTSigningKey  string
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		ServerAddress:  os.Getenv("SERVER_ADDRESS"),
		DbAddress:      os.Getenv("DB_ADDRESS"),
		ScyllaHosts:    os.Getenv("SCYLLA_HOSTS"),
		ScyllaKeyspace: os.Getenv("SCYLLA_KEYSPACE"),
		JWTSigningKey:  os.Getenv("JWT_SIGNING_KEY"),
	}

	if cfg.ServerAddress == "" {
		return nil, fmt.Errorf("SERVER_ADDRESS is required")
	}

	if cfg.DbAddress == "" {
		return nil, fmt.Errorf("DB_ADDRESS is required")
	}

	if cfg.ScyllaHosts == "" {
		return nil, fmt.Errorf("SCYLLA_HOSTS is required")
	}

	if cfg.ScyllaKeyspace == "" {
		return nil, fmt.Errorf("SCYLLA_KEYSPACE is required")
	}

	if cfg.JWTSigningKey == "" {
		return nil, fmt.Errorf("JWT_SIGNING_KEY is required")
	}
	if len(cfg.JWTSigningKey) < minJWTSigningKeyLengthBytes {
		return nil, fmt.Errorf("JWT_SIGNING_KEY must be at least %d bytes", minJWTSigningKeyLengthBytes)
	}

	return cfg, nil
}

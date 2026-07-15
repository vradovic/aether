package realtime

import (
	"fmt"
	"os"
)

const minJWTSigningKeyLengthBytes = 32

type config struct {
	ServerAddress string
	DbAddress     string
	NatsAddress   string
	JWTSigningKey string
	JWTIssuer     string
}

func LoadConfig() (config, error) {
	cfg := config{
		ServerAddress: os.Getenv("SERVER_ADDRESS"),
		DbAddress:     os.Getenv("DB_ADDRESS"),
		NatsAddress:   os.Getenv("NATS_ADDRESS"),
		JWTSigningKey: os.Getenv("JWT_SIGNING_KEY"),
		JWTIssuer:     os.Getenv("JWT_ISSUER"),
	}

	if cfg.ServerAddress == "" {
		return config{}, fmt.Errorf("SERVER_ADDRESS is required")
	}

	if cfg.DbAddress == "" {
		return config{}, fmt.Errorf("DB_ADDRESS is required")
	}

	if cfg.NatsAddress == "" {
		return config{}, fmt.Errorf("NATS_ADDRESS is required")
	}

	if cfg.JWTSigningKey == "" {
		return config{}, fmt.Errorf("JWT_SIGNING_KEY is required")
	}
	if len(cfg.JWTSigningKey) < minJWTSigningKeyLengthBytes {
		return config{}, fmt.Errorf("JWT_SIGNING_KEY must be at least %d bytes", minJWTSigningKeyLengthBytes)
	}

	if cfg.JWTIssuer == "" {
		return config{}, fmt.Errorf("JWT_ISSUER is required")
	}

	return cfg, nil
}

package api

import (
	"fmt"
	"os"
	"time"
)

const minJWTSigningKeyLengthBytes = 32

type Config struct {
	ServerAddress     string
	DbAddress         string
	JWTSigningKey     string
	JWTIssuer         string
	JWTAccessTokenTTL time.Duration
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		ServerAddress: os.Getenv("SERVER_ADDRESS"),
		DbAddress:     os.Getenv("DB_ADDRESS"),
		JWTSigningKey: os.Getenv("JWT_SIGNING_KEY"),
		JWTIssuer:     os.Getenv("JWT_ISSUER"),
	}

	if cfg.ServerAddress == "" {
		return nil, fmt.Errorf("SERVER_ADDRESS is required")
	}

	if cfg.DbAddress == "" {
		return nil, fmt.Errorf("DB_ADDRESS is required")
	}

	if cfg.JWTSigningKey == "" {
		return nil, fmt.Errorf("JWT_SIGNING_KEY is required")
	}
	if len(cfg.JWTSigningKey) < minJWTSigningKeyLengthBytes {
		return nil, fmt.Errorf("JWT_SIGNING_KEY must be at least %d bytes", minJWTSigningKeyLengthBytes)
	}

	if cfg.JWTIssuer == "" {
		return nil, fmt.Errorf("JWT_ISSUER is required")
	}

	jwtAccessTokenTTLValue := os.Getenv("JWT_ACCESS_TOKEN_TTL")
	if jwtAccessTokenTTLValue == "" {
		return nil, fmt.Errorf("JWT_ACCESS_TOKEN_TTL is required")
	}

	jwtAccessTokenTTL, err := time.ParseDuration(jwtAccessTokenTTLValue)
	if err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_TOKEN_TTL must be a valid duration: %w", err)
	}
	if jwtAccessTokenTTL < time.Second {
		return nil, fmt.Errorf("JWT_ACCESS_TOKEN_TTL must be at least one second")
	}
	cfg.JWTAccessTokenTTL = jwtAccessTokenTTL

	return cfg, nil
}

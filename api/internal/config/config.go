package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port string
	DatabaseURL string
	RedisURL string
}

func Load() (Config, error) {
	c := Config{
		Port:        "8080",
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}
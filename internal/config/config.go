package config

import (
	"os"
)

type Config struct {
	Env      string
	Port     string
	LogLevel string
}

func Load() *Config {
	return &Config{
		Env:      getEnv("APP_ENV", "development"),
		Port:     getEnv("APP_PORT", "8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

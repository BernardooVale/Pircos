package config

import (
	"fmt"
	"os"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Port        string
	DatabaseURL string
	StorageDir  string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://pircos:pircos_secret@localhost:5432/pircos_db?sslmode=disable"),
		StorageDir:  getEnv("STORAGE_DIR", "./storage/documents"),
	}
}

// DSN returns the formatted database connection string.
func (c *Config) DSN() string {
	return c.DatabaseURL
}

// Addr returns the formatted listen address (e.g., ":8080").
func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

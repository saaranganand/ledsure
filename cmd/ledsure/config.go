package main

import (
	"fmt"
	"os"
)

// runtime settings
type config struct {
	addr           string
	databaseURL    string
	migrateOnStart bool
	logLevel       string
}

// reads config from env
func loadConfig() (config, error) {
	cfg := config{
		addr:           getEnv("LEDSURE_ADDR", ":8080"),
		migrateOnStart: getEnv("LEDSURE_MIGRATE_ON_START", "false") == "true",
		logLevel:       getEnv("LEDSURE_LOG_LEVEL", "info"),
	}

	cfg.databaseURL = os.Getenv("LEDSURE_DATABASE_URL") // database url must be set explicitly
	if cfg.databaseURL == "" {
		return config{}, fmt.Errorf("LEDSURE_DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

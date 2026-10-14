package config

import (
	"os"
)

type Config struct {
	Port        string
	Environment string
	ServiceName string
	AppVersion  string
}

func LoadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "production"
	}

	return &Config{
		Port:        port,
		Environment: env,
		ServiceName: "dewacell-monolith-api",
		AppVersion:  "1.0.0",
	}
}

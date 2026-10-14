package config

import (
	"os"
)

type Config struct {
	Port              string
	Environment       string
	ServiceName       string
	AppVersion        string
	SupabaseURL       string
	SupabaseAnonKey   string
	SupabaseSecretKey string
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

	supabaseURL := os.Getenv("SUPABASE_URL")
	if supabaseURL == "" {
		supabaseURL = "https://wdpzehzatqdkcslakbiw.supabase.co"
	}

	supabaseAnonKey := os.Getenv("SUPABASE_ANON_KEY")
	if supabaseAnonKey == "" {
		supabaseAnonKey = os.Getenv("SUPABASE_PUBLISHABLE_KEY")
	}
	if supabaseAnonKey == "" {
		supabaseAnonKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6IndkcHplaHphdHFka2NzbGFrYml3Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3NjE0NzYzMTYsImV4cCI6MjA3NzA1MjMxNn0._MzViNByTdyZ9nF9YoHP7PZAoCcI0Lq0i5b8kY0IZAg"
	}

	supabaseSecretKey := os.Getenv("SUPABASE_SECRET_KEY")

	return &Config{
		Port:              port,
		Environment:       env,
		ServiceName:       "dewacell-monolith-api",
		AppVersion:        "1.1.0",
		SupabaseURL:       supabaseURL,
		SupabaseAnonKey:   supabaseAnonKey,
		SupabaseSecretKey: supabaseSecretKey,
	}
}

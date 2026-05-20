// Package config provides application configuration loaded from environment variables.
// For AWS Lambda deployments, environment variables are set via the Lambda console or SAM/CDK templates.
// For local development, a .env file is loaded via godotenv.
package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/joho/godotenv"
)

// Config holds all required environment variables for the calendar agent.
type Config struct {
	// DatabaseURL is the PostgreSQL connection string for Neon DB.
	DatabaseURL string

	// CalendarBaseURL is the base URL of the Calendar API server.
	CalendarBaseURL string

	// ContactBaseURL is the base URL for the contact API endpoints.
	ContactBaseURL string

	// GeminiAPIKey is the API key for Google Gemini.
	GeminiAPIKey string

	// Model is the Gemini model name (e.g., "gemini-2.5-flash").
	Model string
}

var (
	cfg    *Config
	cfgErr error
	once   sync.Once
)

// Load reads environment variables and returns a validated Config.
// It uses sync.Once to ensure the config is loaded only once across
// multiple Lambda invocations within the same execution context.
// If the first load fails, subsequent calls return the same error.
func Load() (*Config, error) {
	once.Do(func() {
		// Attempt to load .env file for local development.
		// Silently ignore if not found (expected in Lambda).
		_ = godotenv.Load()

		c := &Config{
			DatabaseURL:     os.Getenv("DATABASE_URL"),
			CalendarBaseURL: os.Getenv("CALENDAR_BASE_URL"),
			ContactBaseURL:  os.Getenv("CONTACT_BASE_URL"),
			GeminiAPIKey:    os.Getenv("GEMINI_API_KEY"),
			Model:           os.Getenv("MODEL"),
		}

		if err := c.validate(); err != nil {
			cfgErr = err
			return
		}

		cfg = c
	})

	return cfg, cfgErr
}

// validate ensures all required environment variables are set.
func (c *Config) validate() error {
	required := map[string]string{
		"DATABASE_URL":      c.DatabaseURL,
		"CALENDAR_BASE_URL": c.CalendarBaseURL,
		"CONTACT_BASE_URL":  c.ContactBaseURL,
		"GEMINI_API_KEY":    c.GeminiAPIKey,
		"MODEL":             c.Model,
	}

	for name, value := range required {
		if value == "" {
			return fmt.Errorf("config: required environment variable %s is not set", name)
		}
	}

	return nil
}

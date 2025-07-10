package config

import (
	"errors"
	"gopkg.in/yaml.v3"
	"log"
	"os"
)

// Config holds the entire application configuration.
// It uses yaml tags for file-based config and is then overridden by environment variables.
type Config struct {
	Port       string  `yaml:"port"`
	DBHost     string  `yaml:"db_host"`
	DBPort     string  `yaml:"db_port"`
	DBUser     string  `yaml:"db_user"`
	DBPassword string  `yaml:"db_password"` // This will come from env
	DBName     string  `yaml:"db_name"`
	Routes     []Route `yaml:"routes"`
	JWTSecret  string  `yaml:"jwt_secret"` // This will come from env
}

// Route defines a single routing rule
type Route struct {
	PathPrefix  string `yaml:"path_prefix"`
	UpstreamURL string `yaml:"upstream_url"`
}

// LoadConfig reads configuration from a file and overrides with environment variables.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}

	//  Load the base configuration from the YAML file
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(file, cfg)
	if err != nil {
		return nil, err
	}

	//  Override with values from environment variables
	// This allows for secure handling of secrets and environment-specific settings.
	overrideWithEnv(cfg)

	// Validate that essential secrets are present
	if cfg.DBPassword == "" {
		return nil, errors.New("DB_PASSWORD environment variable must be set")
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET environment variable must be set")
	}

	return cfg, nil
}

// overrideWithEnv checks for environment variables and updates the config struct.
func overrideWithEnv(cfg *Config) {
	cfg.Port = getEnv("PORT", cfg.Port)
	cfg.DBHost = getEnv("DB_HOST", cfg.DBHost)
	cfg.DBPort = getEnv("DB_PORT", cfg.DBPort)
	cfg.DBUser = getEnv("DB_USER", cfg.DBUser)
	cfg.DBName = getEnv("DB_NAME", cfg.DBName)

	// For secrets, we don't want a default value from the file, so the second arg is ""
	cfg.DBPassword = getEnv("DB_PASSWORD", "")
	cfg.JWTSecret = getEnv("JWT_SECRET", "")
}

// getEnv retrieves an environment variable or returns a default value.
// It's a robust helper that can handle cases where a variable is set but empty.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		log.Printf("Using environment variable '%s'", key)
		return value
	}
	if defaultValue == "" {
		log.Printf("Warning: Environment variable '%s' not set, and no default value provided.", key)
	}
	return defaultValue
}

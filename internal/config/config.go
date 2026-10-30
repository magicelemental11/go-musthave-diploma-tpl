// Package config loads service settings from flags and environment variables.
package config

import (
	"errors"
	"flag"
	"os"
)

// DefaultSecretKey is the session signing key used when SECRET_KEY is unset.
// It is meant for development only.
const DefaultSecretKey = "dev-secret-key"

const defaultRunAddress = "localhost:8080"

// Config holds the service settings.
type Config struct {
	RunAddress     string
	DatabaseURI    string
	AccrualAddress string
	SecretKey      string
}

// Load reads settings from args and the environment. An environment variable
// wins over a flag, a flag wins over the default. DatabaseURI and
// AccrualAddress are required.
func Load(args []string) (Config, error) {
	var cfg Config

	fs := flag.NewFlagSet("gophermart", flag.ContinueOnError)
	fs.StringVar(&cfg.RunAddress, "a", defaultRunAddress, "service address")
	fs.StringVar(&cfg.DatabaseURI, "d", "", "database URI")
	fs.StringVar(&cfg.AccrualAddress, "r", "", "accrual system address")
	cfg.SecretKey = DefaultSecretKey
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	override(&cfg.RunAddress, "RUN_ADDRESS")
	override(&cfg.DatabaseURI, "DATABASE_URI")
	override(&cfg.AccrualAddress, "ACCRUAL_SYSTEM_ADDRESS")
	override(&cfg.SecretKey, "SECRET_KEY")

	if cfg.DatabaseURI == "" {
		return Config{}, errors.New("database URI is required: set DATABASE_URI or -d")
	}
	if cfg.AccrualAddress == "" {
		return Config{}, errors.New("accrual system address is required: set ACCRUAL_SYSTEM_ADDRESS or -r")
	}
	return cfg, nil
}

func override(dst *string, key string) {
	if v := os.Getenv(key); v != "" {
		*dst = v
	}
}

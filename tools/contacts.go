package main

import (
	"os"
	"strings"

	"deanprice.com/internal/defaults"
)

func getEnvOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		val = strings.Trim(val, `"'`)
		if val != "" {
			return val
		}
	}
	return fallback
}

// Config encapsulates build-time contact configurations.
type Config struct {
	Email   string
	PhoneGB string
	PhoneIE string
	PhoneTR string
}

// LoadConfigFromEnv reads contact settings from environment variables or falls back to defaults.
func LoadConfigFromEnv() Config {
	return Config{
		Email:   getEnvOrDefault("SECRET_EMAIL", defaults.DefaultEmail),
		PhoneGB: getEnvOrDefault("SECRET_PHONE_GB", defaults.DefaultPhoneGB),
		PhoneIE: getEnvOrDefault("SECRET_PHONE_IE", defaults.DefaultPhoneIE),
		PhoneTR: getEnvOrDefault("SECRET_PHONE_TR", defaults.DefaultPhoneTR),
	}
}

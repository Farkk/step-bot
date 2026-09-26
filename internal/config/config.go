package config

import (
	"fmt"
	"os"
)

type Config struct {
	Env, Port, DatabaseURL, S3Endpoint, WebhookSecret, BotToken string
}

func Load() (Config, error) {
	c := Config{
		Env:           os.Getenv("APP_ENV"),
		Port:          os.Getenv("APP_PORT"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		S3Endpoint:    os.Getenv("S3_ENDPOINT"),
		WebhookSecret: os.Getenv("MAX_WEBHOOK_SECRET"),
		BotToken:      os.Getenv("MAX_BOT_TOKEN"),
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.Env == "" {
		c.Env = "local"
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.S3Endpoint == "" {
		return c, fmt.Errorf("S3_ENDPOINT is required")
	}
	if c.WebhookSecret == "" {
		return c, fmt.Errorf("MAX_WEBHOOK_SECRET is required")
	}
	return c, nil
}

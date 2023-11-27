package config

import (
	"github.com/joho/godotenv"
)

type Config struct {
	WEBSOCKET_URL string
}

func LoadConfig() error {
	if err := godotenv.Load(); err != nil {
		return err
	}
	return nil
}

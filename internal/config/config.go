package config

import (
	"log"
	"os"
)

type Configuration struct {
	AdminToken  string
	DatabaseURL string
}

var Config Configuration

func LoadConfig() {
	Config = Configuration{
		AdminToken:  os.Getenv("ADMIN_TOKEN"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if Config.AdminToken == "" || Config.DatabaseURL == "" {
		log.Fatal("Environment variables ADMIN_TOKEN and DATABASE_URL must be set")
	}
}

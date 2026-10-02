package main

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	TargetURL    string
	PollInterval time.Duration
	DBPath       string
	Port         string
	SiteName     string
	SiteURL      string
	AdminToken   string // enables the maintenance admin API (admin build only)
}

func loadConfig() Config {
	secs, _ := strconv.Atoi(env("POLL_INTERVAL", "30"))
	if secs < 10 {
		secs = 10
	}
	return Config{
		TargetURL:    env("TARGET_URL", "http://localhost:3000/api/health"),
		PollInterval: time.Duration(secs) * time.Second,
		DBPath:       env("DB_PATH", "./data/monitor.db"),
		Port:         env("PORT", "8080"),
		SiteName:     env("SITE_NAME", "Meetify"),
		SiteURL:      env("SITE_URL", ""),
		AdminToken:   env("ADMIN_TOKEN", ""),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package main

import (
	"os"
	"path/filepath"
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
	Domain       string // when set, serve HTTPS on :443 with an auto-renewed Let's Encrypt certificate
	ACMEEmail    string // optional contact address for Let's Encrypt expiry notices
	CertDir      string // autocert cache; must live on a persistent volume
}

func loadConfig() Config {
	dbPath := env("DB_PATH", "./data/monitor.db")
	secs, _ := strconv.Atoi(env("POLL_INTERVAL", "30"))
	if secs < 10 {
		secs = 10
	}
	return Config{
		TargetURL:    env("TARGET_URL", "http://localhost:3000/api/health"),
		PollInterval: time.Duration(secs) * time.Second,
		DBPath:       dbPath,
		Port:         env("PORT", "8080"),
		SiteName:     env("SITE_NAME", "Meetify"),
		SiteURL:      env("SITE_URL", ""),
		AdminToken:   env("ADMIN_TOKEN", ""),
		Domain:       env("DOMAIN", ""),
		ACMEEmail:    env("ACME_EMAIL", ""),
		CertDir:      env("CERT_DIR", filepath.Join(filepath.Dir(dbPath), "certs")),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

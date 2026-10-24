package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"
)

const (
	// A single reset or timeout (seen about once per 1000 polls through Cloudflare)
	// must not open an incident, so a failed check is retried before it counts.
	checkAttempts = 3
	retryDelay    = 3 * time.Second
)

func runPoller(db *sql.DB, cfg Config) {
	// Keep-alive stays on so the response time is the request itself, not TCP+TLS every poll.
	client := &http.Client{Timeout: 10 * time.Second}

	probe := func() (status string, code, elapsed int, err error) {
		start := time.Now()
		resp, err := client.Get(cfg.TargetURL)
		elapsed = int(time.Since(start).Milliseconds())
		if err != nil {
			return "down", 0, elapsed, err
		}
		resp.Body.Close()
		code = resp.StatusCode
		if code >= 500 {
			return "down", code, elapsed, nil
		}
		return "up", code, elapsed, nil
	}

	check := func() {
		var (
			status        string
			code, elapsed int
			err           error
		)
		for attempt := 1; attempt <= checkAttempts; attempt++ {
			status, code, elapsed, err = probe()
			if status == "up" {
				break
			}
			if err != nil {
				log.Printf("poll FAIL  attempt=%d/%d err=%v", attempt, checkAttempts, err)
			} else {
				log.Printf("poll FAIL  attempt=%d/%d %d  %dms", attempt, checkAttempts, code, elapsed)
			}
			if attempt < checkAttempts {
				client.CloseIdleConnections() // a retry must not reuse the connection that just failed
				time.Sleep(retryDelay)
			}
		}
		if status == "up" {
			log.Printf("poll %-4s  %d  %dms", status, code, elapsed)
		}

		if err := recordCheck(db, status, elapsed, code); err != nil {
			log.Printf("recordCheck: %v", err)
		}
	}

	check() // immediate first check on startup

	t := time.NewTicker(cfg.PollInterval)
	defer t.Stop()
	for range t.C {
		check()
	}
}

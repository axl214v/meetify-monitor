package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"
)

func runPoller(db *sql.DB, cfg Config) {
	client := &http.Client{Timeout: 10 * time.Second}

	check := func() {
		start := time.Now()
		resp, err := client.Get(cfg.TargetURL)
		elapsed := int(time.Since(start).Milliseconds())

		status := "up"
		code := 0

		if err != nil {
			status = "down"
			log.Printf("poll FAIL  err=%v", err)
		} else {
			resp.Body.Close()
			code = resp.StatusCode
			if code >= 500 {
				status = "down"
			}
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

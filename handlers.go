package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

type pageData struct {
	SiteName    string
	IsUp        bool
	LastChecked time.Time
	ResponseMs  int
	UptimeStats []UptimeStat
	DailyStatus []DayStatus
	Incidents   []Incident
}

func handleIndex(db *sql.DB, cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		cur := currentStatus(db)
		data := pageData{
			SiteName:    cfg.SiteName,
			IsUp:        cur.Status == "up",
			LastChecked: cur.CheckedAt,
			ResponseMs:  cur.ResponseMs,
			UptimeStats: uptimeStats(db),
			DailyStatus: dailyStatus(db),
			Incidents:   recentIncidents(db, 20),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

type apiStatus struct {
	Status      string    `json:"status"`
	LastChecked time.Time `json:"last_checked"`
	ResponseMs  int       `json:"response_ms"`
	UptimePct   struct {
		H24 float64 `json:"24h"`
		D7  float64 `json:"7d"`
		D30 float64 `json:"30d"`
	} `json:"uptime_pct"`
}

func handleAPIStatus(db *sql.DB, _ Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cur := currentStatus(db)
		stats := uptimeStats(db)

		resp := apiStatus{
			Status:      cur.Status,
			LastChecked: cur.CheckedAt,
			ResponseMs:  cur.ResponseMs,
		}
		if len(stats) == 3 {
			resp.UptimePct.H24 = stats[0].Pct
			resp.UptimePct.D7 = stats[1].Pct
			resp.UptimePct.D30 = stats[2].Pct
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(resp)
	}
}

package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

func openDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite doesn't support concurrent writes
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS checks (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			checked_at  TEXT    NOT NULL,
			status      TEXT    NOT NULL,
			response_ms INTEGER,
			http_code   INTEGER
		);
		CREATE TABLE IF NOT EXISTS incidents (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at  TEXT NOT NULL,
			resolved_at TEXT,
			duration_s  INTEGER
		);
		CREATE INDEX IF NOT EXISTS idx_checks_at ON checks(checked_at);
	`)
	return err
}

func recordCheck(db *sql.DB, status string, responseMs, httpCode int) error {
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := db.Exec(
		`INSERT INTO checks (checked_at, status, response_ms, http_code) VALUES (?, ?, ?, ?)`,
		now, status, responseMs, httpCode,
	); err != nil {
		return err
	}

	if status == "down" {
		var open int
		db.QueryRow(`SELECT COUNT(*) FROM incidents WHERE resolved_at IS NULL`).Scan(&open)
		if open == 0 {
			_, err := db.Exec(`INSERT INTO incidents (started_at) VALUES (?)`, now)
			return err
		}
		return nil
	}

	// up — close any open incident
	_, err := db.Exec(`
		UPDATE incidents
		SET resolved_at = ?,
		    duration_s  = CAST((julianday(?) - julianday(started_at)) * 86400 AS INTEGER)
		WHERE resolved_at IS NULL`,
		now, now,
	)
	return err
}

// ── query types ──────────────────────────────────────────────────────────────

type Check struct {
	Status     string
	ResponseMs int
	CheckedAt  time.Time
}

func currentStatus(db *sql.DB) Check {
	var c Check
	var at string
	db.QueryRow(
		`SELECT status, response_ms, checked_at FROM checks ORDER BY checked_at DESC LIMIT 1`,
	).Scan(&c.Status, &c.ResponseMs, &at)
	c.CheckedAt, _ = time.Parse(time.RFC3339, at)
	return c
}

type UptimeStat struct {
	Period string
	Pct    float64
}

func uptimeStats(db *sql.DB) []UptimeStat {
	periods := []struct {
		label string
		days  int
	}{
		{"24h", 1}, {"7d", 7}, {"30d", 30},
	}
	out := make([]UptimeStat, 0, len(periods))
	for _, p := range periods {
		var total, up int
		db.QueryRow(
			`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='up' THEN 1 ELSE 0 END), 0)
			 FROM checks WHERE checked_at >= datetime('now', ?)`,
			fmt.Sprintf("-%d days", p.days),
		).Scan(&total, &up)
		pct := 100.0
		if total > 0 {
			pct = float64(up) / float64(total) * 100
		}
		out = append(out, UptimeStat{p.label, pct})
	}
	return out
}

type DayStatus struct {
	Date     string // YYYY-MM-DD
	HasData  bool
	AllUp    bool
	HasIssue bool
}

func dailyStatus(db *sql.DB) []DayStatus {
	rows, err := db.Query(`
		SELECT date(checked_at) AS day,
		       COUNT(*) AS total,
		       SUM(CASE WHEN status='up' THEN 1 ELSE 0 END) AS up_count
		FROM checks
		WHERE checked_at >= datetime('now', '-90 days')
		GROUP BY day`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	byDay := map[string]DayStatus{}
	for rows.Next() {
		var day string
		var total, upCount int
		rows.Scan(&day, &total, &upCount)
		byDay[day] = DayStatus{
			Date:     day,
			HasData:  true,
			AllUp:    upCount == total,
			HasIssue: upCount < total,
		}
	}

	result := make([]DayStatus, 90)
	for i := 0; i < 90; i++ {
		day := time.Now().UTC().AddDate(0, 0, -(89 - i)).Format("2006-01-02")
		if s, ok := byDay[day]; ok {
			result[i] = s
		} else {
			result[i] = DayStatus{Date: day}
		}
	}
	return result
}

type Incident struct {
	StartedAt  time.Time
	ResolvedAt *time.Time
	DurationS  *int
}

func recentIncidents(db *sql.DB, limit int) []Incident {
	rows, err := db.Query(
		`SELECT started_at, resolved_at, duration_s FROM incidents ORDER BY started_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Incident
	for rows.Next() {
		var inc Incident
		var startedAt string
		var resolvedAt sql.NullString
		var durationS sql.NullInt64
		rows.Scan(&startedAt, &resolvedAt, &durationS)
		inc.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		if resolvedAt.Valid {
			t, _ := time.Parse(time.RFC3339, resolvedAt.String)
			inc.ResolvedAt = &t
		}
		if durationS.Valid {
			d := int(durationS.Int64)
			inc.DurationS = &d
		}
		out = append(out, inc)
	}
	return out
}

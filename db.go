package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
		CREATE TABLE IF NOT EXISTS maintenances (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			title     TEXT NOT NULL,
			starts_at TEXT NOT NULL,
			ends_at   TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_checks_at ON checks(checked_at);
	`)
	return err
}

func recordCheck(db *sql.DB, status string, responseMs, httpCode int) error {
	now := time.Now().UTC().Format(time.RFC3339)

	// A failure inside a maintenance window is expected, not an outage.
	if status == "down" && activeMaintenance(db) != nil {
		status = "maintenance"
	}

	if _, err := db.Exec(
		`INSERT INTO checks (checked_at, status, response_ms, http_code) VALUES (?, ?, ?, ?)`,
		now, status, responseMs, httpCode,
	); err != nil {
		return err
	}

	if status == "maintenance" {
		return nil
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
	HTTPCode   int
	CheckedAt  time.Time
}

func currentStatus(db *sql.DB) Check {
	var c Check
	var at string
	db.QueryRow(
		`SELECT status, COALESCE(response_ms, 0), COALESCE(http_code, 0), checked_at FROM checks ORDER BY checked_at DESC LIMIT 1`,
	).Scan(&c.Status, &c.ResponseMs, &c.HTTPCode, &at)
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
			 FROM checks WHERE status != 'maintenance' AND checked_at >= datetime('now', ?)`,
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
	Date  string // YYYY-MM-DD
	Pct   float64
	State string // "up", "partial", "down", "maint" or "" when there is no data
	Maint bool   // the day contained maintenance checks
}

// dayState maps a day's uptime to a timeline colour: 100% is up,
// 95% and above is a partial degradation, anything lower is down.
func dayState(pct float64) string {
	switch {
	case pct >= 100:
		return "up"
	case pct >= 95:
		return "partial"
	default:
		return "down"
	}
}

func dailyStatus(db *sql.DB) []DayStatus {
	rows, err := db.Query(`
		SELECT date(checked_at) AS day,
		       COUNT(*) AS total,
		       SUM(CASE WHEN status='up' THEN 1 ELSE 0 END) AS up_count,
		       SUM(CASE WHEN status='maintenance' THEN 1 ELSE 0 END) AS maint_count
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
		var total, upCount, maintCount int
		rows.Scan(&day, &total, &upCount, &maintCount)
		// Maintenance checks don't count against the day's uptime.
		if counted := total - maintCount; counted > 0 {
			pct := float64(upCount) / float64(counted) * 100
			byDay[day] = DayStatus{Date: day, Pct: pct, State: dayState(pct), Maint: maintCount > 0}
		} else {
			byDay[day] = DayStatus{Date: day, Pct: 100, State: "maint", Maint: true}
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
	HTTPCode   int // code of the first failed check, 0 if unknown
}

func recentIncidents(db *sql.DB, limit int) []Incident {
	rows, err := db.Query(
		`SELECT started_at, resolved_at, duration_s,
		        COALESCE((SELECT http_code FROM checks WHERE checked_at = incidents.started_at LIMIT 1), 0)
		 FROM incidents ORDER BY started_at DESC LIMIT ?`,
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
		rows.Scan(&startedAt, &resolvedAt, &durationS, &inc.HTTPCode)
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

// ── response times ───────────────────────────────────────────────────────────

type ResponseStats struct {
	AvgMs int
	P95Ms int
}

type Spark struct {
	Line  string    // polyline points
	Area  string    // polygon points (line closed down to the baseline)
	Down  []float64 // x positions of buckets that contained failed checks
	MaxMs int
	Width int
	High  int
}

const (
	sparkBuckets = 48 // 30-minute buckets over 24h
	sparkW       = 480
	sparkH       = 60
)

// responseTimes returns avg/p95 response time and a sparkline for the last 24h.
func responseTimes(db *sql.DB) (ResponseStats, Spark) {
	spark := Spark{Width: sparkW, High: sparkH}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	rows, err := db.Query(
		`SELECT checked_at, status, COALESCE(response_ms, 0) FROM checks WHERE checked_at >= ? ORDER BY checked_at`,
		cutoff.Format(time.RFC3339),
	)
	if err != nil {
		return ResponseStats{}, spark
	}
	defer rows.Close()

	var all []int
	var sum [sparkBuckets]int
	var cnt [sparkBuckets]int
	var down [sparkBuckets]bool
	bucketLen := 24 * time.Hour / sparkBuckets
	for rows.Next() {
		var at, status string
		var ms int
		rows.Scan(&at, &status, &ms)
		t, err := time.Parse(time.RFC3339, at)
		if err != nil {
			continue
		}
		b := int(t.Sub(cutoff) / bucketLen)
		if b < 0 || b >= sparkBuckets {
			continue
		}
		if status == "maintenance" {
			continue
		}
		if status != "up" {
			down[b] = true
			continue
		}
		all = append(all, ms)
		sum[b] += ms
		cnt[b]++
	}

	var stats ResponseStats
	if len(all) > 0 {
		total := 0
		for _, v := range all {
			total += v
		}
		stats.AvgMs = total / len(all)
		sort.Ints(all)
		stats.P95Ms = all[(len(all)*95+99)/100-1]
	}

	for b := 0; b < sparkBuckets; b++ {
		if cnt[b] > 0 {
			if avg := sum[b] / cnt[b]; avg > spark.MaxMs {
				spark.MaxMs = avg
			}
		}
	}
	if spark.MaxMs == 0 {
		spark.MaxMs = 1
	}
	step := float64(sparkW) / float64(sparkBuckets-1)
	var line []string
	var firstX, lastX float64
	for b := 0; b < sparkBuckets; b++ {
		x := float64(b) * step
		if down[b] {
			spark.Down = append(spark.Down, x)
		}
		if cnt[b] == 0 {
			continue
		}
		y := float64(sparkH-4) - float64(sum[b]/cnt[b])/float64(spark.MaxMs)*float64(sparkH-8)
		if len(line) == 0 {
			firstX = x
		}
		lastX = x
		line = append(line, fmt.Sprintf("%.1f,%.1f", x, y))
	}
	if len(line) >= 2 {
		spark.Line = strings.Join(line, " ")
		spark.Area = fmt.Sprintf("%s %.1f,%d %.1f,%d", spark.Line, lastX, sparkH, firstX, sparkH)
	}
	return stats, spark
}

// upSince returns when the current uninterrupted up streak began: the end of
// the last incident, or the very first check if there has never been one.
func upSince(db *sql.DB) time.Time {
	var s sql.NullString
	db.QueryRow(`SELECT MAX(resolved_at) FROM incidents`).Scan(&s)
	if !s.Valid {
		db.QueryRow(`SELECT MIN(checked_at) FROM checks`).Scan(&s)
	}
	t, _ := time.Parse(time.RFC3339, s.String)
	return t
}

// ── maintenance windows ──────────────────────────────────────────────────────

type Maintenance struct {
	ID       int64     `json:"id"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

func (m Maintenance) Active() bool {
	now := time.Now()
	return !now.Before(m.StartsAt) && now.Before(m.EndsAt)
}

func addMaintenance(db *sql.DB, title string, start, end time.Time) (Maintenance, error) {
	m := Maintenance{Title: title, StartsAt: start.UTC(), EndsAt: end.UTC()}
	res, err := db.Exec(
		`INSERT INTO maintenances (title, starts_at, ends_at) VALUES (?, ?, ?)`,
		title, m.StartsAt.Format(time.RFC3339), m.EndsAt.Format(time.RFC3339),
	)
	if err != nil {
		return m, err
	}
	m.ID, _ = res.LastInsertId()
	return m, nil
}

func deleteMaintenance(db *sql.DB, id int64) (bool, error) {
	res, err := db.Exec(`DELETE FROM maintenances WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func queryMaintenances(db *sql.DB, query string, args ...any) []Maintenance {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Maintenance
	for rows.Next() {
		var m Maintenance
		var s, e string
		rows.Scan(&m.ID, &m.Title, &s, &e)
		m.StartsAt, _ = time.Parse(time.RFC3339, s)
		m.EndsAt, _ = time.Parse(time.RFC3339, e)
		out = append(out, m)
	}
	return out
}

// upcomingMaintenances returns active and future windows, soonest first.
func upcomingMaintenances(db *sql.DB) []Maintenance {
	return queryMaintenances(db,
		`SELECT id, title, starts_at, ends_at FROM maintenances WHERE ends_at > ? ORDER BY starts_at`,
		time.Now().UTC().Format(time.RFC3339))
}

// pastMaintenances returns the most recently finished windows.
func pastMaintenances(db *sql.DB, limit int) []Maintenance {
	return queryMaintenances(db,
		`SELECT id, title, starts_at, ends_at FROM maintenances WHERE ends_at <= ? ORDER BY ends_at DESC LIMIT ?`,
		time.Now().UTC().Format(time.RFC3339), limit)
}

// activeMaintenance returns the window in progress right now, or nil.
func activeMaintenance(db *sql.DB) *Maintenance {
	now := time.Now().UTC().Format(time.RFC3339)
	ms := queryMaintenances(db,
		`SELECT id, title, starts_at, ends_at FROM maintenances WHERE starts_at <= ? AND ends_at > ? ORDER BY ends_at DESC LIMIT 1`,
		now, now)
	if len(ms) == 0 {
		return nil
	}
	return &ms[0]
}

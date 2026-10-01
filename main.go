package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"
)

//go:embed templates/index.html static
var assetFS embed.FS

var tmpl *template.Template

func main() {
	cfg := loadConfig()

	var err error
	tmpl, err = template.New("index.html").Funcs(template.FuncMap{
		"fmtDuration": fmtDuration,
		"fmtTime":     fmtTime,
		"ago":         ago,
		"since":       since,
		"unix":        func(t time.Time) int64 { return t.Unix() },
	}).ParseFS(assetFS, "templates/index.html")
	if err != nil {
		log.Fatal("parse template: ", err)
	}

	db, err := openDB(cfg.DBPath)
	if err != nil {
		log.Fatal("open db: ", err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatal("migrate: ", err)
	}

	go runPoller(db, cfg)

	mux := http.NewServeMux()
	mux.Handle("/static/", cacheStatic(http.FileServerFS(assetFS)))
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/logo-mark.png", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/", handleIndex(db, cfg))
	mux.HandleFunc("/api/status", handleAPIStatus(db, cfg))

	log.Printf("meetify-monitor listening on :%s -> polling %s every %s", cfg.Port, cfg.TargetURL, cfg.PollInterval)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}

func cacheStatic(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		h.ServeHTTP(w, r)
	})
}

// since formats the time elapsed since t as "3d 4h", "5h 12m" or "7m".
func since(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd %dh", int(d.Hours()/24), int(d.Hours())%24)
	}
}

func fmtDuration(s *int) string {
	if s == nil {
		return "ongoing"
	}
	d := *s
	switch {
	case d < 60:
		return fmt.Sprintf("%ds", d)
	case d < 3600:
		return fmt.Sprintf("%dm %ds", d/60, d%60)
	default:
		return fmt.Sprintf("%dh %dm", d/3600, (d%3600)/60)
	}
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2, 2006 15:04 UTC")
}

func ago(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

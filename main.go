package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"
)

//go:embed templates/index.html
var templateFS embed.FS

var tmpl *template.Template

func main() {
	cfg := loadConfig()

	var err error
	tmpl, err = template.New("index.html").Funcs(template.FuncMap{
		"fmtDuration": fmtDuration,
		"fmtTime":     fmtTime,
		"ago":         ago,
	}).ParseFS(templateFS, "templates/index.html")
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
	mux.HandleFunc("/", handleIndex(db, cfg))
	mux.HandleFunc("/api/status", handleAPIStatus(db, cfg))

	log.Printf("meetify-monitor listening on :%s -> polling %s every %s", cfg.Port, cfg.TargetURL, cfg.PollInterval)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
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

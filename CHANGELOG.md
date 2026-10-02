# Changelog

All notable changes to meetify-monitor are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
(while in early development, breaking changes may still land in minor releases).

## [Unreleased]

### Added
- Scheduled maintenance windows: failed checks inside a window are recorded as `maintenance` (no incident, excluded from uptime), the page shows a maintenance banner, upcoming windows and blue days in the history. `GET /api/maintenances`, `status: "maintenance"` in `/api/status`.
- Optional private admin build (`-tags admin`, `ADMIN_TOKEN`) for managing windows; the public build ships a no-op `registerAdmin`.
- Status page UI: Meetify logo/favicon (embedded `/static`), Open Graph tags, dark/light theme toggle (choice saved in the browser), mobile layout.
- Status page: HTTP code, uptime streak, avg/p95 response time, 24h response sparkline, 3-colour day history with per-day uptime, incident end time and HTTP code.
- Status page refreshes in place every 30s without a full reload.
- `SITE_URL` env var; `avg_response_ms_24h` / `p95_response_ms_24h` in `/api/status`.
- Initial release: Go poller + SQLite storage + status page.
- `GET /` status page — current state, 24h/7d/30d uptime, 90-day history bar, incident log.
- `GET /api/status` JSON endpoint.
- Docker / Docker Compose deployment, single-container, no external dependencies.

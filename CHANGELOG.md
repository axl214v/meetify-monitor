# Changelog

All notable changes to meetify-monitor are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
(while in early development, breaking changes may still land in minor releases).

## [Unreleased]

### Added
- Initial release: Go poller + SQLite storage + status page.
- `GET /` status page — current state, 24h/7d/30d uptime, 90-day history bar, incident log.
- `GET /api/status` JSON endpoint.
- Docker / Docker Compose deployment, single-container, no external dependencies.

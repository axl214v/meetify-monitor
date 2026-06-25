<div align="center">
  <p><strong>Independent uptime monitor for Meetify</strong><br/>
  Polls a health endpoint · SQLite-backed history · public status page</p>

  <p>
    <a href="https://meetify.cc">meetify.cc</a> &nbsp;·&nbsp;
    <a href="https://github.com/axl214v/Meetify">Meetify</a> &nbsp;·&nbsp;
    <a href="CHANGELOG.md">Changelog</a>
  </p>

  [![License](https://img.shields.io/badge/license-Polyform%20Noncommercial-orange)](LICENSE.md)
  [![Go](https://img.shields.io/badge/go-%3E%3D1.23-00ADD8)](https://go.dev/)
  [![Docker](https://img.shields.io/badge/Docker-Compose-blue)](https://www.docker.com/)
</div>

---

## What is meetify-monitor?

A small, independent Go service that polls a health endpoint on an interval
and serves a public status page — current state, 24h/7d/30d uptime, a
90-day history bar, and an incident log.

It is deliberately decoupled from the Meetify stack — meant to run on a
separate host so it keeps reporting status even if the Meetify server, or
the machine it runs on, goes down entirely.

## Features

- Polls any HTTP health endpoint on a configurable interval
- SQLite storage (pure Go, no CGO) — single binary, single-file DB
- Status page styled to match Meetify's design system
- `GET /api/status` JSON endpoint for external integrations
- Single static binary, small Docker image, no external dependencies

## Self-Hosting

**Requirements:** Docker + Docker Compose.

```bash
git clone https://github.com/axl214v/meetify-monitor.git
cd meetify-monitor

cp .env.example .env   # set TARGET_URL to the service you're monitoring
docker compose up -d --build
```

Open **http://localhost:8080**.

### Configuration (`.env`)

| Variable | Default | Description |
|---|---|---|
| `TARGET_URL` | `http://localhost:3000/api/health` | Endpoint to poll |
| `POLL_INTERVAL` | `30` | Seconds between checks (minimum 10) |
| `SITE_NAME` | `Meetify` | Display name on the status page |
| `PORT` | `8080` | HTTP listen port |
| `DB_PATH` | `./data/monitor.db` | SQLite file path |

## Technology Stack

| Layer | Technology |
|---|---|
| Language | Go 1.23 |
| Storage | SQLite (`modernc.org/sqlite`, no CGO) |
| Web | `net/http`, `html/template` |
| Infrastructure | Docker, single-container deploy |

## Architecture

```
poller (goroutine, every N seconds)
  → GET TARGET_URL
  → SQLite: checks + incidents

HTTP server
  → GET /              status page (HTML)
  → GET /api/status    current status + uptime % (JSON)
```

Run it behind your own reverse proxy (Nginx, Caddy, Cloudflare) for TLS and
to put it on a subdomain like `health.meetify.cc`.

## Roadmap

- [ ] Multi-target monitoring (more than one URL per instance)
- [ ] Webhook/email alert on status change
- [ ] Response-time graph on the status page

## Contributing

Pull requests are welcome — see [contributing.md](contributing.md).

## License

Licensed under the **Polyform Noncommercial License 1.0.0** — see [LICENSE.md](LICENSE.md).

- **Personal / educational / self-hosted use:** free
- **Commercial use:** requires a separate license — [legal@meetify.cc](mailto:legal@meetify.cc)

---

<div align="center">
  <sub>Built by <a href="https://github.com/axl214v">axl214v</a> · part of the <a href="https://github.com/axl214v/Meetify">Meetify</a> project</sub>
</div>

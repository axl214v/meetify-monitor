# Security Policy

## Supported Versions

meetify-monitor is in active early development. Security fixes land on the
**latest** `main` only — there are no release/LTS branches yet.

## Reporting a Vulnerability

**Please do not open public GitHub issues for security problems.**

Report privately by email to **security@meetify.cc** with the subject
`SECURITY: meetify-monitor`. Where possible include:

- A description of the issue and its impact.
- Steps to reproduce (a proof of concept is ideal).
- The affected commit and your environment.
- Any suggested remediation.

**What to expect:**
- Acknowledgement within **72 hours**.
- An initial assessment (accepted / needs-info / declined) within **7 days**.
- For accepted reports, a fix or mitigation plan, and credit in the release
  notes if you'd like it.

Please give us reasonable time to release a fix before any public disclosure
(coordinated disclosure).

## Security Measures in Place

- **No authentication, no user accounts, no PII.** The service stores only
  poll timestamps, HTTP status codes, and response times for the configured
  `TARGET_URL`.
- **Read-only public surface.** The status page (`/`) and `/api/status` are
  intentionally public; there is no write path exposed over HTTP.
- **Single dependency for storage** (`modernc.org/sqlite`, pure Go, no CGO) —
  the database file lives in a Docker volume and is never exposed externally.

## Known Limitations (by design / not yet implemented)

- **No rate limiting** on `/` or `/api/status` — both are cheap, read-only
  SQLite queries, but putting a reverse proxy (Nginx/Cloudflare) in front is
  recommended for abuse protection and TLS termination.
- **HTTPS is not handled by this service** — terminate TLS at a reverse proxy.
- **No built-in alerting** — incidents are recorded and shown on the status
  page, but nothing pages anyone yet (see Roadmap in [readme.md](readme.md)).

## Scope

In scope: this repository (Go source, Dockerfile, templates). Out of scope:
the application being monitored — for Meetify itself, see
[Meetify's SECURITY.md](https://github.com/axl214v/Meetify/blob/main/SECURITY.md).

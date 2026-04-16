# Novex Backend (Go)

Go backend for NovexPanel with JWT auth, agent management over WebSocket, real-time metrics, terminal proxy, process control, and deploy flow.

## What Is Implemented

- User registration and login (`/auth/register`, `/auth/login`)
- Agent token management (`POST/GET/PATCH/DELETE /auth/tokens`)
- Agent WebSocket (`/agent/ws?token=...`)
- Site WebSocket (`/site/ws?token=...`)
- Server list + latest metrics (`GET /servers`)
- Metrics history for 7 days (`GET /servers/:id/metrics?interval=1s|1m|5m|1h`, default: `1s`)
- Process list and kill process (`GET /servers/:id/processes`, `DELETE /servers/:id/processes/:pid`)
- Command execution (`POST /servers/:id/command`)
- Deploy request + logs (`POST /servers/:id/deploy`, `GET /deploys/:id/logs`)

## Project Layout

- `cmd/server` - backend API + WebSocket gateway
- `cmd/agent` - Linux agent binary
- `internal/...` - config, auth, models, app logic
- `docs/openapi.yaml` - REST API specification
- `docs/ws-protocol.md` - WebSocket protocol details

## Quick Start

### 1) Start PostgreSQL (optional but recommended)

```bash
docker compose up -d postgres
```

### 2) Configure env

```bash
cp .env.example .env
```

If you want fast local MVP without Postgres, set:

```env
DATABASE_URL=novex.db
```

This runs SQLite and can later migrate to Postgres by changing `DATABASE_URL`.

### 3) Install Go deps

```bash
go mod tidy
```

### 4) Run backend

```bash
go run ./cmd/server
```

By default backend runs on `:8080`.

### 5) Create user and get JWT

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"secret123"}'

curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"secret123"}'
```

### 6) Generate agent token

```bash
curl -X POST http://localhost:8080/auth/tokens \
  -H "Authorization: Bearer <JWT>" \
  -H "Content-Type: application/json" \
  -d '{"name":"thinkpad"}'
```

You can rename an existing token later:

```bash
curl -X PATCH http://localhost:8080/auth/tokens/<TOKEN_ID> \
  -H "Authorization: Bearer <JWT>" \
  -H "Content-Type: application/json" \
  -d '{"name":"new-name"}'
```

### 7) Run agent on Linux server

```bash
go run ./cmd/agent -backend ws://<backend-host>:8080 -token <AGENT_TOKEN> -name "server-1"
```

Agent auto reconnects and sends metrics every 2 seconds.

## Build Binaries

Backend server:

```bash
go build -o novex-server ./cmd/server
```

Agent (Linux):

```bash
GOOS=linux GOARCH=amd64 go build -o novex-agent ./cmd/agent
```

## Security Notes

- Use HTTPS/WSS in production (behind reverse proxy like Nginx/Caddy)
- Set strong `JWT_SECRET`
- Restrict CORS with `SITE_ALLOWED_ORIGINS`
- Agent token is stored as SHA-256 hash in DB

## Frontend Integration

- REST contract: `docs/openapi.yaml`
- WebSocket protocol: `docs/ws-protocol.md`

These documents are enough to implement auth pages, dashboard, metrics chart, terminal, deploy form, process list, and deploy logs.

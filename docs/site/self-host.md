# Self-Hosting

Deploy Garance on your own infrastructure with Docker Compose. One file, all services, full control.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose v2+
- A Linux server (or macOS/Windows for development)
- At least 2 GB RAM

## Quick start

```bash
git clone https://github.com/garancehq/garance.git
cd garance/deploy

# Create your environment file
cp .env.example .env

# Edit .env with secure values
nano .env

# Start all services
docker compose up -d
```

Your Garance instance is now running:

| Service          | URL                      |
|------------------|--------------------------|
| **API Gateway**  | http://localhost:8080     |
| **Dashboard**    | http://localhost:3000     |

## Architecture

```
Client (SDK / Dashboard)
       |
       v :8080
   ┌──────────┐
   │ Gateway  │  HTTP → gRPC routing, CORS, JWT validation
   └──┬─┬─┬───┘
      │ │ │
      v v v
   ┌──┐┌──┐┌───┐
   │E ││A ││ S │  Engine (Rust) / Auth (Go) / Storage (Go)
   └┬─┘└┬─┘└┬──┘
    │   │   │
    v   v   v
  ┌─────────┐ ┌───────┐
  │ Postgres│ │ MinIO │
  └─────────┘ └───────┘
```

| Service    | Language | Port (HTTP) | Port (gRPC) | Role                                        |
|------------|----------|-------------|-------------|---------------------------------------------|
| Gateway    | Go       | 8080        | --          | HTTP proxy, CORS, JWT validation, routing    |
| Engine     | Rust     | 4000        | 5000        | Query builder, introspection, migrations     |
| Auth       | Go       | 4001        | 5001        | Sign-up, sign-in, JWT, sessions              |
| Storage    | Go       | 4002        | 5002        | File upload/download, signed URLs, buckets   |
| Dashboard  | Next.js  | 3000        | --          | Admin UI                                     |
| PostgreSQL | --       | 5432        | --          | Database                                     |
| MinIO      | --       | 9000/9001   | --          | S3-compatible object storage                 |

Internal communication between Gateway and services uses **gRPC**. The Gateway is the only service exposed to the internet.

## Environment variables

### Required

| Variable        | Description                                   | Example                         |
|-----------------|-----------------------------------------------|---------------------------------|
| `DB_PASSWORD`   | PostgreSQL password                           | `supersecretpassword`           |
| `JWT_SECRET`    | Secret key for JWT signing (min 32 chars)     | `openssl rand -base64 32`       |

### Recommended

| Variable        | Description                                   | Default           |
|-----------------|-----------------------------------------------|-------------------|
| `DB_NAME`       | PostgreSQL database name                      | `garance`         |
| `S3_ACCESS_KEY` | MinIO access key                              | `minioadmin`      |
| `S3_SECRET_KEY` | MinIO secret key                              | `minioadmin`      |
| `S3_BUCKET`     | Default storage bucket name                   | `garance`         |

### Ports

| Variable          | Description                | Default |
|-------------------|----------------------------|---------|
| `GATEWAY_PORT`    | External Gateway port      | `8080`  |
| `DASHBOARD_PORT`  | External Dashboard port    | `3000`  |

### CORS

| Variable          | Description                             | Default |
|-------------------|-----------------------------------------|---------|
| `ALLOWED_ORIGINS` | Comma-separated allowed origins         | `*`     |

### SMTP (optional)

Required for email verification and magic links.

| Variable    | Description             | Default                |
|-------------|-------------------------|------------------------|
| `SMTP_HOST` | SMTP server hostname    | _(none)_               |
| `SMTP_PORT` | SMTP server port        | `587`                  |
| `SMTP_USER` | SMTP username           | _(none)_               |
| `SMTP_PASS` | SMTP password           | _(none)_               |
| `SMTP_FROM` | Sender email address    | `noreply@garance.io`   |

### Version pinning

| Variable           | Description                         | Default  |
|--------------------|-------------------------------------|----------|
| `GARANCE_VERSION`  | Docker image tag for all services   | `latest` |

## Docker Compose file

The production `docker-compose.yml` defines 7 services:

```yaml
services:
  postgres:      # PostgreSQL 17 Alpine
  minio:         # S3-compatible storage
  engine:        # Rust query engine
  auth:          # Go auth service
  storage:       # Go storage service
  gateway:       # Go API gateway (only public-facing service)
  dashboard:     # Next.js admin UI
```

Services are connected via an internal Docker network (`garance`). Only the Gateway (port 8080) and Dashboard (port 3000) are exposed externally.

### Volumes

| Volume            | Description                    |
|-------------------|--------------------------------|
| `garance_data`    | PostgreSQL data                |
| `garance_storage` | MinIO file storage             |

## Reverse proxy

In production, place a reverse proxy (nginx, Caddy, Traefik) in front of the Gateway and Dashboard for TLS termination.

### Caddy example

```
api.garance.example.com {
    reverse_proxy localhost:8080
}

admin.garance.example.com {
    reverse_proxy localhost:3000
}
```

### nginx example

```nginx
server {
    listen 443 ssl;
    server_name api.garance.example.com;

    ssl_certificate     /etc/ssl/certs/garance.pem;
    ssl_certificate_key /etc/ssl/private/garance.key;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Backup

### PostgreSQL

```bash
docker compose exec postgres pg_dump -U postgres garance > backup.sql
```

### Restore

```bash
docker compose exec -T postgres psql -U postgres garance < backup.sql
```

### MinIO

MinIO data is stored in the `garance_storage` Docker volume. Back up the volume or use MinIO's `mc mirror` tool for replication.

## Updating

```bash
cd deploy

# Pull latest images
docker compose pull

# Restart with new images
docker compose up -d
```

To pin a specific version, set `GARANCE_VERSION` in your `.env` file:

```
GARANCE_VERSION=0.1.0
```

## Database schemas

Garance uses separate PostgreSQL schemas for isolation:

| Schema              | Description               |
|---------------------|---------------------------|
| `garance_auth`      | Users, sessions, tokens   |
| `garance_storage`   | Buckets, file metadata    |
| `garance_platform`  | Project configuration     |
| `garance_audit`     | Audit trail               |
| `public`            | Your application tables   |

Your tables (defined in `garance.schema.ts`) live in the `public` schema.

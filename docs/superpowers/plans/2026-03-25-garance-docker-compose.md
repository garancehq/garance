# Garance Docker Compose — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the production self-host Docker Compose configuration and a dev-mode compose for local development. This is the final assembly — wires all services (Engine, Auth, Storage, Gateway, Dashboard, PostgreSQL, MinIO) into a working stack.

**Architecture:** Two compose files: `deploy/docker-compose.yml` (production self-host using published images) and `deploy/docker-compose.dev.yml` (local dev building from source). Both use the same service topology with environment variable configuration.

**Spec:** `docs/superpowers/specs/2026-03-25-garance-baas-design.md` (sections 11, 12)

---

## Task 1: Production Docker Compose

**Files:**
- Create: `deploy/docker-compose.yml`
- Create: `deploy/.env.example`
- Create: `deploy/README.md`

- [ ] **Step 1: Write production compose**

```yaml
# deploy/docker-compose.yml
# Garance BaaS — Self-host production deployment
# Usage:
#   cp .env.example .env
#   # Edit .env with your values
#   docker compose up -d

services:
  postgres:
    image: postgres:17-alpine
    restart: unless-stopped
    volumes:
      - garance_data:/var/lib/postgresql/data
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: ${DB_NAME:-garance}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 10
    networks:
      - garance

  minio:
    image: minio/minio
    restart: unless-stopped
    command: server /data --console-address ":9001"
    volumes:
      - garance_storage:/data
    environment:
      MINIO_ROOT_USER: ${S3_ACCESS_KEY:-minioadmin}
      MINIO_ROOT_PASSWORD: ${S3_SECRET_KEY:-minioadmin}
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - garance

  engine:
    image: ghcr.io/garancehq/engine:${GARANCE_VERSION:-latest}
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgresql://postgres:${DB_PASSWORD}@postgres:5432/${DB_NAME:-garance}
      LISTEN_ADDR: 0.0.0.0:4000
      GRPC_ADDR: 0.0.0.0:5000
    networks:
      - garance

  auth:
    image: ghcr.io/garancehq/auth:${GARANCE_VERSION:-latest}
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgresql://postgres:${DB_PASSWORD}@postgres:5432/${DB_NAME:-garance}
      LISTEN_ADDR: 0.0.0.0:4001
      GRPC_ADDR: 0.0.0.0:5001
      JWT_SECRET: ${JWT_SECRET}
      SMTP_HOST: ${SMTP_HOST:-}
      SMTP_PORT: ${SMTP_PORT:-587}
      SMTP_USER: ${SMTP_USER:-}
      SMTP_PASS: ${SMTP_PASS:-}
      SMTP_FROM: ${SMTP_FROM:-noreply@garance.io}
    networks:
      - garance

  storage:
    image: ghcr.io/garancehq/storage:${GARANCE_VERSION:-latest}
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
      minio:
        condition: service_healthy
    environment:
      DATABASE_URL: postgresql://postgres:${DB_PASSWORD}@postgres:5432/${DB_NAME:-garance}
      LISTEN_ADDR: 0.0.0.0:4002
      GRPC_ADDR: 0.0.0.0:5002
      S3_ENDPOINT: minio:9000
      S3_ACCESS_KEY: ${S3_ACCESS_KEY:-minioadmin}
      S3_SECRET_KEY: ${S3_SECRET_KEY:-minioadmin}
      S3_BUCKET: ${S3_BUCKET:-garance}
      JWT_SECRET: ${JWT_SECRET}
    networks:
      - garance

  gateway:
    image: ghcr.io/garancehq/gateway:${GARANCE_VERSION:-latest}
    restart: unless-stopped
    ports:
      - "${GATEWAY_PORT:-8080}:8080"
    depends_on:
      - engine
      - auth
      - storage
    environment:
      LISTEN_ADDR: 0.0.0.0:8080
      ENGINE_GRPC_ADDR: engine:5000
      AUTH_GRPC_ADDR: auth:5001
      STORAGE_GRPC_ADDR: storage:5002
      JWT_SECRET: ${JWT_SECRET}
      ALLOWED_ORIGINS: ${ALLOWED_ORIGINS:-*}
    networks:
      - garance

  dashboard:
    image: ghcr.io/garancehq/dashboard:${GARANCE_VERSION:-latest}
    restart: unless-stopped
    ports:
      - "${DASHBOARD_PORT:-3000}:3000"
    depends_on:
      - gateway
    environment:
      NEXT_PUBLIC_GATEWAY_URL: http://gateway:8080
    networks:
      - garance

volumes:
  garance_data:
  garance_storage:

networks:
  garance:
    driver: bridge
```

- [ ] **Step 2: Write .env.example**

```bash
# deploy/.env.example
# Garance BaaS — Environment Configuration
# Copy this file to .env and fill in the values

# Database
DB_PASSWORD=CHANGE_ME_TO_A_STRONG_PASSWORD
DB_NAME=garance

# JWT Secret (generate with: openssl rand -base64 32)
JWT_SECRET=CHANGE_ME_TO_A_RANDOM_SECRET

# S3 Storage (MinIO)
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=CHANGE_ME_TO_A_STRONG_PASSWORD
S3_BUCKET=garance

# Ports (external)
GATEWAY_PORT=8080
DASHBOARD_PORT=3000

# CORS
ALLOWED_ORIGINS=*

# SMTP (optional, for email verification and magic links)
# SMTP_HOST=smtp.example.com
# SMTP_PORT=587
# SMTP_USER=
# SMTP_PASS=
# SMTP_FROM=noreply@yourdomain.com

# Version pinning (default: latest)
# GARANCE_VERSION=0.1.0
```

- [ ] **Step 3: Write README**

```markdown
# Garance — Self-Hosted Deployment

## Quick Start

```bash
# 1. Copy and edit environment variables
cp .env.example .env
# Edit .env — at minimum change DB_PASSWORD, JWT_SECRET, and S3_SECRET_KEY

# 2. Start all services
docker compose up -d

# 3. Verify
docker compose ps
curl http://localhost:8080/health
```

## Services

| Service | Internal Port | External Port | Description |
|---------|--------------|---------------|-------------|
| Gateway | 8080 | 8080 | API entry point |
| Dashboard | 3000 | 3000 | Admin UI |
| Engine | 4000/5000 | — | Query engine (HTTP/gRPC) |
| Auth | 4001/5001 | — | Authentication (HTTP/gRPC) |
| Storage | 4002/5002 | — | File storage (HTTP/gRPC) |
| PostgreSQL | 5432 | — | Database |
| MinIO | 9000/9001 | — | S3-compatible storage |

## Management

```bash
# View logs
docker compose logs -f

# View specific service
docker compose logs -f gateway

# Stop
docker compose down

# Stop and remove volumes (⚠️ destroys data)
docker compose down -v

# Update to latest version
docker compose pull
docker compose up -d
```

## Configuration

All configuration is done through environment variables in `.env`. See `.env.example` for available options.
```

- [ ] **Step 4: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add deploy/
git commit -m ":whale: feat(deploy): add production Docker Compose for self-host deployment"
```

---

## Task 2: Dev Docker Compose

**Files:**
- Create: `deploy/docker-compose.dev.yml`

- [ ] **Step 1: Write dev compose (builds from source)**

```yaml
# deploy/docker-compose.dev.yml
# Garance BaaS — Local development (builds from source)
# Usage: docker compose -f docker-compose.dev.yml up --build

services:
  postgres:
    image: postgres:17-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: garance
    volumes:
      - garance_dev_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 2s
      timeout: 5s
      retries: 10

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    volumes:
      - garance_dev_storage:/data

  engine:
    build:
      context: ../engine
      dockerfile: Dockerfile
    ports:
      - "4000:4000"
      - "5000:5000"
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgresql://postgres:postgres@postgres:5432/garance
      LISTEN_ADDR: 0.0.0.0:4000
      GRPC_ADDR: 0.0.0.0:5000

  auth:
    build:
      context: ../services/auth
      dockerfile: Dockerfile
    ports:
      - "4001:4001"
      - "5001:5001"
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgresql://postgres:postgres@postgres:5432/garance
      LISTEN_ADDR: 0.0.0.0:4001
      GRPC_ADDR: 0.0.0.0:5001
      JWT_SECRET: dev-secret-change-me

  storage:
    build:
      context: ../services/storage
      dockerfile: Dockerfile
    ports:
      - "4002:4002"
      - "5002:5002"
    depends_on:
      postgres:
        condition: service_healthy
      minio:
        condition: service_started
    environment:
      DATABASE_URL: postgresql://postgres:postgres@postgres:5432/garance
      LISTEN_ADDR: 0.0.0.0:4002
      GRPC_ADDR: 0.0.0.0:5002
      S3_ENDPOINT: minio:9000
      S3_ACCESS_KEY: minioadmin
      S3_SECRET_KEY: minioadmin
      JWT_SECRET: dev-secret-change-me

  gateway:
    build:
      context: ../services/gateway
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    depends_on:
      - engine
      - auth
      - storage
    environment:
      LISTEN_ADDR: 0.0.0.0:8080
      ENGINE_GRPC_ADDR: engine:5000
      AUTH_GRPC_ADDR: auth:5001
      STORAGE_GRPC_ADDR: storage:5002
      JWT_SECRET: dev-secret-change-me
      ALLOWED_ORIGINS: "*"

  dashboard:
    build:
      context: ../dashboard
      dockerfile: Dockerfile
    ports:
      - "3000:3000"
    depends_on:
      - gateway
    environment:
      NEXT_PUBLIC_GATEWAY_URL: http://localhost:8080

volumes:
  garance_dev_data:
  garance_dev_storage:
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add deploy/
git commit -m ":whale: feat(deploy): add dev Docker Compose for building from source"
```

---

## Summary

| Task | Description | Files |
|---|---|---|
| 1 | Production compose (published images) + .env.example + README | 3 |
| 2 | Dev compose (builds from source) | 1 |

### Architecture

```
Client (browser/SDK)
    │
    ▼ :8080
┌─────────────┐
│   Gateway   │ ← HTTP entry point
└──┬──┬──┬────┘
   │  │  │  gRPC
   ▼  ▼  ▼
┌──┐ ┌──┐ ┌───┐
│E │ │A │ │ S │  Engine / Auth / Storage
└┬─┘ └┬─┘ └┬──┘
 │    │    │
 ▼    ▼    ▼
┌──────────┐  ┌───────┐
│PostgreSQL│  │ MinIO │
└──────────┘  └───────┘

Dashboard (:3000) → Gateway (:8080)
```

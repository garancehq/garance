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

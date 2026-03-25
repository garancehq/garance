# Garance — Claude Code Instructions

## Project

Garance is an open source Backend-as-a-Service (BaaS). PostgreSQL + auto-generated REST API + Auth + Storage + Dashboard. Declarative schema in TypeScript.

## Structure

```
garance/
├── engine/           # Rust — Query Engine (axum + tonic)
├── services/
│   ├── auth/         # Go — Auth (JWT, argon2id, sessions)
│   ├── storage/      # Go — Storage (S3/MinIO)
│   └── gateway/      # Go — API Gateway (HTTP→gRPC)
├── cli/              # Go — CLI (cobra)
├── packages/
│   └── schema/       # TypeScript — @garance/schema DSL
├── sdks/
│   └── typescript/   # TypeScript — @garance/sdk
├── dashboard/        # Next.js 16 — Admin UI (shadcn/ui)
├── proto/            # Protobuf — gRPC contracts
└── deploy/           # Docker Compose
```

## Conventions

### Languages & Tools

- **Rust**: Cargo workspace, edition 2021, stable toolchain
- **Go**: Go workspace (`services/go.work`), standard modules, `go test`
- **TypeScript**: tsup (bundling), vitest (tests), npm
- **Next.js**: App Router, Server Components, shadcn/ui, Tailwind CSS, dark mode
- **Proto**: buf for linting, protoc-gen-go + protoc-gen-go-grpc for Go, tonic-build for Rust

### Git

- Commit format: gitmoji + conventional commit (e.g., `:sparkles: feat(engine): add query builder`)
- Never add Co-Authored-By Claude or any AI mention in commits
- One commit per atomic feature

### Architecture

- Inter-service communication: gRPC (proto definitions in `proto/`)
- Each service exposes HTTP + gRPC on separate ports
- Ports: Engine 4000/5000, Auth 4001/5001, Storage 4002/5002, Gateway 8080, Dashboard 3000
- Standardized error format: `{"error": {"code": "...", "message": "...", "status": N}}`
- Database: PostgreSQL 17, separate schemas (`garance_auth`, `garance_storage`, `garance_platform`, `garance_audit`)

### Testing

- **Rust**: `cargo test` (testcontainers for integration tests)
- **Go**: `go test ./... -count=1` (testcontainers-go for PostgreSQL and MinIO)
- **TypeScript**: `vitest run`
- Docker must be running for integration tests (testcontainers)

### Design (Dashboard)

- Dark mode by default (zinc-950 bg, zinc-100 text, zinc-800 borders)
- Geist Sans for text, font-mono for data/code
- shadcn/ui for all UI components

## Useful Commands

```bash
# Engine
cd engine && cargo test
cd engine && cargo build -p garance-engine

# Go services (auth, storage, gateway)
cd services/auth && go test ./... -count=1
cd services/auth && go build ./...

# CLI
cd cli && go build -o garance .
./garance --help

# Schema (@garance/schema)
cd packages/schema && npm test && npm run build

# SDK (@garance/sdk)
cd sdks/typescript && npm test && npm run build

# Dashboard
cd dashboard && npm run build

# Proto (regenerate code)
cd proto && buf generate

# Docker Compose (dev)
cd deploy && docker compose -f docker-compose.dev.yml up --build

# Docker Compose (prod)
cd deploy && docker compose up -d
```

## Specs & Plans

- Design spec: `docs/superpowers/specs/2026-03-25-garance-baas-design.md`
- Implementation plans: `docs/superpowers/plans/2026-03-25-garance-*.md`

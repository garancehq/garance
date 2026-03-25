# Garance — Instructions pour Claude Code

## Projet

Garance est un BaaS (Backend-as-a-Service) souverain open source, alternative europeenne a Supabase. PostgreSQL + API REST auto-generee + Auth + Storage + Dashboard.

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
├── proto/            # Protobuf — contrats gRPC
└── deploy/           # Docker Compose
```

## Conventions

### Langages et outils

- **Rust** : Cargo workspace, edition 2021, stable toolchain
- **Go** : Go workspace (`services/go.work`), modules standards, `go test`
- **TypeScript** : tsup (bundling), vitest (tests), pnpm/npm
- **Next.js** : App Router, Server Components, shadcn/ui, Tailwind CSS, dark mode
- **Proto** : buf pour le linting, protoc-gen-go + protoc-gen-go-grpc pour la generation Go, tonic-build pour Rust

### Git

- Format de commit : gitmoji + conventional commit (ex: `:sparkles: feat(engine): add query builder`)
- Ne jamais ajouter de Co-Authored-By Claude ni de mention Claude/AI dans les commits
- Un commit par fonctionnalite atomique

### Architecture

- Communication inter-services : gRPC (proto definitions dans `proto/`)
- Chaque service expose HTTP + gRPC sur des ports separes
- Ports : Engine 4000/5000, Auth 4001/5001, Storage 4002/5002, Gateway 8080, Dashboard 3000
- Format d'erreur standardise : `{"error": {"code": "...", "message": "...", "status": N}}`
- Base de donnees : PostgreSQL 17, schemas separes (`garance_auth`, `garance_storage`, `garance_platform`, `garance_audit`)

### Tests

- **Rust** : `cargo test` (testcontainers pour les tests d'integration)
- **Go** : `go test ./... -count=1` (testcontainers-go pour PostgreSQL et MinIO)
- **TypeScript** : `vitest run`
- Docker doit etre demarre pour les tests d'integration (testcontainers)

### Design (Dashboard)

- Dark mode par defaut (zinc-950 bg, zinc-100 text, zinc-800 borders)
- Geist Sans pour le texte, font-mono pour les donnees/code
- shadcn/ui pour tous les composants UI
- Pas d'emoji dans le code ou les fichiers sauf demande explicite

## Commandes utiles

```bash
# Engine
cd engine && cargo test
cd engine && cargo build -p garance-engine

# Services Go (auth, storage, gateway)
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

# Proto (regenerer le code)
cd proto && buf generate

# Docker Compose (dev)
cd deploy && docker compose -f docker-compose.dev.yml up --build

# Docker Compose (prod)
cd deploy && docker compose up -d
```

## Specs et plans

- Design spec : `docs/superpowers/specs/2026-03-25-garance-baas-design.md`
- Plans d'implementation : `docs/superpowers/plans/2026-03-25-garance-*.md`

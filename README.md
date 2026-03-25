# Garance

**Open source Backend-as-a-Service** — A sovereign alternative to Supabase, built in Europe.

Garance provides a managed PostgreSQL database with auto-generated REST API, authentication, file storage, and an admin dashboard. GDPR-compliant by design, open source by conviction.

## Architecture

```
Client (SDK / Dashboard)
       |
       v :8080
   ┌──────────┐
   │ Gateway  │  HTTP → gRPC
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

| Service | Language | Role |
|---------|----------|------|
| Engine | Rust | PG introspection, auto-generated REST API, query builder, type codegen |
| Auth | Go | Signup/signin, JWT, sessions, argon2id |
| Storage | Go | S3 upload/download, buckets, signed URLs |
| Gateway | Go | HTTP → gRPC proxy, CORS, JWT, routing |
| CLI | Go | `garance init`, `garance dev`, `garance db migrate` |
| Schema | TypeScript | Declarative DSL (`garance.schema.ts` → JSON) |
| SDK | TypeScript | Client SDK (`createClient()` with auth, data, storage) |
| Dashboard | Next.js | Admin UI (table editor, SQL editor, users, storage, settings) |

## Quick Start

### Self-host (Docker Compose)

```bash
cd deploy
cp .env.example .env
# Edit .env (DB_PASSWORD, JWT_SECRET, S3_SECRET_KEY)
docker compose up -d
```

Available services:
- **API Gateway**: http://localhost:8080
- **Dashboard**: http://localhost:3000

### Local Development (CLI)

```bash
# Install the CLI
brew install garancehq/tap/garance

# Initialize a project
garance init my-project
cd my-project

# Start the dev environment
garance dev

# Manage the database
garance db migrate
garance db seed
garance db reset

# Generate client types
garance gen types --lang ts
```

### TypeScript SDK

```typescript
import { createClient } from '@garance/sdk'

const garance = createClient({ url: 'http://localhost:8080' })

// Auth
await garance.auth.signUp({ email: 'dev@example.com', password: 'password' })
await garance.auth.signIn({ email: 'dev@example.com', password: 'password' })

// Data
const { data } = await garance.from('users').eq('name', 'Alice').limit(10).execute()

// Storage
await garance.storage.from('avatars').upload('photo.jpg', file)
const url = garance.storage.from('avatars').getPublicUrl('photo.jpg')
```

### Declarative Schema

```typescript
// garance.schema.ts
import { defineSchema, table, column, relation } from '@garance/schema'

export default defineSchema({
  users: table({
    id: column.uuid().primaryKey().default('gen_random_uuid()'),
    email: column.text().unique().notNull(),
    name: column.text().notNull(),
    posts: relation.hasMany('posts', 'author_id'),
  }),
  posts: table({
    id: column.uuid().primaryKey().default('gen_random_uuid()'),
    title: column.text().notNull(),
    author_id: column.uuid().references('users.id'),
    published: column.bool().default('false'),
    access: {
      read: (ctx) => ctx.where({ published: true }).or(ctx.isOwner('author_id')),
      write: (ctx) => ctx.isOwner('author_id'),
    },
  }),
})
```

## Repository Structure

```
garance/
├── engine/           # Query Engine (Rust, 3 crates)
├── services/
│   ├── auth/         # Auth Service (Go)
│   ├── storage/      # Storage Service (Go)
│   └── gateway/      # API Gateway (Go)
├── cli/              # CLI (Go)
├── packages/
│   └── schema/       # @garance/schema (TypeScript)
├── sdks/
│   └── typescript/   # @garance/sdk (TypeScript)
├── dashboard/        # Admin Dashboard (Next.js)
├── proto/            # gRPC proto definitions
└── deploy/           # Docker Compose (prod + dev)
```

## Testing

```bash
# Engine (Rust)
cd engine && cargo test

# Auth (Go) — requires Docker
cd services/auth && go test ./... -count=1

# Storage (Go) — requires Docker
cd services/storage && go test ./... -count=1

# Schema (TypeScript)
cd packages/schema && npm test

# SDK (TypeScript)
cd sdks/typescript && npm test
```

## Tech Stack

- **Rust**: Engine (axum, tonic, tokio-postgres, deadpool)
- **Go**: Auth, Storage, Gateway, CLI (pgx, grpc, cobra, minio-go)
- **TypeScript**: Schema DSL, SDK, Dashboard (Next.js, shadcn/ui)
- **PostgreSQL 17**: Database
- **MinIO**: S3-compatible object storage
- **gRPC**: Inter-service communication (protobuf)

## Why Garance

- **Open source** (Apache 2.0) — self-host anywhere, audit everything
- **GDPR-native** — account deletion, data export, audit trail built in
- **Declarative schema** — define your database in TypeScript, not raw SQL
- **Polyglot architecture** — each service in the best language for the job
- **European infrastructure** — default hosting on Scaleway (Paris), no US dependency in the critical path

## License

[Apache License 2.0](LICENSE)

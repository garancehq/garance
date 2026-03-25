# Garance

**BaaS souverain open source** — Alternative europenne a Supabase.

Garance fournit une base de donnees PostgreSQL avec API REST auto-generee, authentification, stockage de fichiers, et un dashboard d'administration. Heberge en France, conforme RGPD.

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

| Service | Langage | Role |
|---------|---------|------|
| Engine | Rust | Introspection PG, API REST auto-generee, query builder, codegen types |
| Auth | Go | Signup/signin, JWT, sessions, argon2id |
| Storage | Go | Upload/download S3, buckets, signed URLs |
| Gateway | Go | HTTP → gRPC proxy, CORS, JWT, routing |
| CLI | Go | `garance init`, `garance dev`, `garance db migrate` |
| Schema | TypeScript | DSL declaratif (`garance.schema.ts` → JSON) |
| SDK | TypeScript | Client SDK (`createClient()` avec auth, data, storage) |
| Dashboard | Next.js | Admin UI (table editor, SQL editor, users, storage, settings) |

## Quick Start

### Self-host (Docker Compose)

```bash
cd deploy
cp .env.example .env
# Editez .env (DB_PASSWORD, JWT_SECRET, S3_SECRET_KEY)
docker compose up -d
```

Services disponibles :
- **API Gateway** : http://localhost:8080
- **Dashboard** : http://localhost:3000

### Developpement local (CLI)

```bash
# Installer la CLI
brew install garancehq/tap/garance

# Initialiser un projet
garance init mon-projet
cd mon-projet

# Lancer l'environnement
garance dev

# Gerer la base de donnees
garance db migrate
garance db seed
garance db reset

# Generer les types
garance gen types --lang ts
```

### SDK TypeScript

```typescript
import { createClient } from '@garance/sdk'

const garance = createClient({ url: 'http://localhost:8080' })

// Auth
await garance.auth.signUp({ email: 'dev@example.fr', password: 'motdepasse' })
await garance.auth.signIn({ email: 'dev@example.fr', password: 'motdepasse' })

// Data
const { data } = await garance.from('users').eq('name', 'Alice').limit(10).execute()

// Storage
await garance.storage.from('avatars').upload('photo.jpg', file)
const url = garance.storage.from('avatars').getPublicUrl('photo.jpg')
```

### Schema declaratif

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

## Structure du repo

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

## Tests

```bash
# Engine (Rust)
cd engine && cargo test

# Auth (Go) — necessite Docker
cd services/auth && go test ./... -count=1

# Storage (Go) — necessite Docker
cd services/storage && go test ./... -count=1

# Schema (TypeScript)
cd packages/schema && npm test

# SDK (TypeScript)
cd sdks/typescript && npm test
```

## Tech Stack

- **Rust** : Engine (axum, tonic, tokio-postgres, deadpool)
- **Go** : Auth, Storage, Gateway, CLI (pgx, grpc, cobra, minio-go)
- **TypeScript** : Schema DSL, SDK, Dashboard (Next.js, shadcn/ui)
- **PostgreSQL 17** : Base de donnees
- **MinIO** : Stockage S3-compatible
- **gRPC** : Communication inter-services (protobuf)

## Souverainete

- Hebergement cible : Scaleway (Paris)
- Aucune dependance a des services US dans le chemin critique
- RGPD natif (suppression de compte, export des donnees, audit trail)
- Open source (Apache 2.0)

## Licence

[Apache License 2.0](LICENSE)

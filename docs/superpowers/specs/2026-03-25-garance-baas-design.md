# Garance — BaaS Souverain Français/Européen

> Document de design — 25 mars 2026

## 1. Vision

Garance est un Backend-as-a-Service (BaaS) open source, souverain, positionné comme alternative européenne à Supabase. Le projet vise à contribuer à la souveraineté numérique française et européenne en offrant une plateforme complète, performante et respectueuse du RGPD.

## 2. Modèle

**Open-core** :
- Cœur open source et self-hostable (Docker Compose, Helm Chart)
- SaaS managé sur `garance.io` pour les revenus récurrents

| | Open Source | SaaS Managé |
|---|---|---|
| Cœur (DB, auth, API, storage) | Oui | Oui |
| Dashboard admin | Oui | Oui |
| Self-host mono/multi-projets | Oui (manuel) | — |
| Provisioning automatisé | Non | Oui |
| Scaling / monitoring | Manuel | Inclus |
| Backups automatiques | Non | Oui |
| Support | Communauté | Prioritaire |
| Certifications (SecNumCloud, HDS) | Non | Oui |

**Pricing SaaS** : plans fixes + overage (Free → Pro → Team → Enterprise).

## 3. Architecture Globale

```
┌─────────────────────────────────────────────────────┐
│                    Clients                          │
│  @garance/sdk (TS) · garance_dart · garance_swift   │
└──────────────┬──────────────────────┬───────────────┘
               │ HTTPS                │ HTTPS
┌──────────────▼──────────────────────▼───────────────┐
│              API Gateway (Go)                       │
│  Routing · Rate limiting · Auth middleware · CORS    │
└──┬──────────┬──────────────┬───────────────┬────────┘
   │          │              │               │
┌──▼───┐  ┌──▼───────┐  ┌───▼────────┐  ┌───▼──────┐
│ Auth │  │ Query    │  │ Storage    │  │Dashboard │
│ (Go) │  │ Engine   │  │ Service    │  │ (Next.js)│
│      │  │ (Rust)   │  │ (Go)       │  │          │
└──┬───┘  └──┬───────┘  └───┬────────┘  └──────────┘
   │         │              │
   │    ┌────▼────────┐  ┌──▼──────────────┐
   │    │ Connection  │  │ S3-compatible    │
   │    │ Pooler      │  │ (Scaleway/MinIO) │
   │    │ (Rust)      │  └─────────────────┘
   │    └────┬────────┘
   │         │
┌──▼─────────▼──┐
│  PostgreSQL   │
└───────────────┘
```

Communication inter-services : gRPC entre les services Go et le Query Engine Rust. Le Connection Pooler est une lib Rust intégrée au Query Engine.

## 4. Stack Technique

| Couche | Langage | Justification |
|---|---|---|
| Query Engine, Connection Pooler, Codegen | Rust | Perf, sécurité mémoire, chemin critique data |
| Auth, Storage, API Gateway, CLI | Go | Services réseau performants, binaires cross-platform |
| Dashboard, SDK clients, docs | TypeScript | DX, écosystème web, cible principale des utilisateurs |

Architecture polyglotte ciblée : chaque module dans le langage le plus adapté. La complexité est transparente pour l'utilisateur final (Docker Compose).

## 5. Query Engine (Rust)

### Responsabilités

- Introspection du schéma PostgreSQL (tables, colonnes, types, relations, contraintes, indexes)
- Génération automatique d'API REST (CRUD par table)
- Parsing et validation des requêtes → SQL sûr
- Connection pooling intégré
- Lecture du schema déclaratif `garance.schema.ts` et génération de migrations
- Génération de types multi-langage (TypeScript, Dart, Swift, Kotlin)

### API REST auto-générée

```
GET    /api/{table}                    → liste (filtres, pagination, tri)
GET    /api/{table}/{id}               → un enregistrement
POST   /api/{table}                    → insertion
PATCH  /api/{table}/{id}               → mise à jour partielle
DELETE /api/{table}/{id}               → suppression
GET    /api/{table}/{id}/{relation}    → relations imbriquées
POST   /api/rpc/{function}            → appel de fonctions PG
```

### Filtrage

```
GET /api/users?age=gte.18&city=eq.Paris&select=id,name,email&order=name.asc&limit=20
```

### Schema Déclaratif (`garance.schema.ts`)

```typescript
import { defineSchema, table, column, relation } from '@garance/schema'

export default defineSchema({
  users: table({
    id: column.uuid().primaryKey().default('gen_random_uuid()'),
    email: column.text().unique().notNull(),
    name: column.text().notNull(),
    created_at: column.timestamp().default('now()'),
    posts: relation.hasMany('posts', 'author_id'),
  }),

  posts: table({
    id: column.uuid().primaryKey().default('gen_random_uuid()'),
    title: column.text().notNull(),
    content: column.text(),
    author_id: column.uuid().references('users.id'),
    published: column.boolean().default(false),

    access: {
      read: (ctx) => ctx.where({ published: true }).or(ctx.isOwner('author_id')),
      write: (ctx) => ctx.isOwner('author_id'),
      delete: (ctx) => ctx.isOwner('author_id'),
    }
  }),
})
```

### Différenciation vs Supabase

- Permissions déclarées dans le schema (lisibles) au lieu de RLS policies SQL (verbeuses)
- Le schema génère les migrations ET les types clients automatiquement
- Un seul fichier pour comprendre toute la structure de la DB
- Le schema reste optionnel — on peut créer les tables en SQL et l'API se génère par introspection

## 6. Auth Service (Go)

### Méthodes d'authentification

| Méthode | Priorité |
|---|---|
| Email / mot de passe | MVP |
| Magic link (email) | MVP |
| OAuth2 (Google, GitHub, GitLab) | MVP |
| FranceConnect | v1 |
| SAML / SSO entreprise | v1 |
| Passkeys / WebAuthn | v1 |
| SMS / OTP | v2 |

### Architecture

```
Client SDK
    │
    ▼
API Gateway ──► Auth Service (Go)
                    │
                    ├── JWT issuer (access + refresh tokens)
                    ├── Password hashing (argon2id)
                    ├── OAuth2 client (Google, GitHub, GitLab)
                    ├── FranceConnect client (OpenID Connect)
                    ├── Email sender (SMTP / API)
                    │       └── Templates : vérification, reset, magic link
                    └── Sessions table (PostgreSQL)
```

### Tokens

- **Access token** : JWT signé, 15 min, contient `user_id`, `project_id`, `role`
- **Refresh token** : opaque, stocké en DB, 30 jours, rotation à chaque usage
- Le Query Engine lit le JWT pour appliquer les permissions du schema

### SDK

```typescript
const garance = createClient({ url: 'https://mon-projet.garance.io' })

await garance.auth.signUp({ email: 'dev@example.fr', password: '...' })
await garance.auth.signIn({ email: 'dev@example.fr', password: '...' })
await garance.auth.signInWithMagicLink({ email: 'dev@example.fr' })
await garance.auth.signInWithOAuth({ provider: 'google' })

const user = await garance.auth.getUser()
await garance.auth.signOut()
```

### Stockage

Tables auth dans un schéma PostgreSQL séparé (`garance_auth`) : `users`, `sessions`, `identities`, `verification_tokens`.

## 7. Storage Service (Go)

### Architecture

```
Client SDK
    │
    ▼
API Gateway ──► Storage Service (Go)
                    │
                    ├── Bucket management (CRUD)
                    ├── Upload (multipart, jusqu'à 5 Go)
                    ├── Signed URLs (accès temporaire, expiration configurable)
                    ├── Image pipeline (resize, webp, avif via libvips)
                    └── S3 client ──► Scaleway Object Storage (SaaS)
                                  ──► MinIO (self-host)
```

### Permissions dans le schema

```typescript
storage: {
  avatars: bucket({
    maxFileSize: '5mb',
    allowedMimeTypes: ['image/jpeg', 'image/png', 'image/webp'],
    access: {
      read: 'public',
      write: (ctx) => ctx.isAuthenticated(),
      delete: (ctx) => ctx.isOwner(),
    }
  }),
  documents: bucket({
    maxFileSize: '50mb',
    access: {
      read: (ctx) => ctx.isAuthenticated(),
      write: (ctx) => ctx.isAuthenticated(),
    }
  }),
}
```

### SDK

```typescript
// Upload
await garance.storage.from('avatars').upload('user-123/photo.jpg', file)

// URL publique
garance.storage.from('avatars').getPublicUrl('user-123/photo.jpg')

// URL signée
await garance.storage.from('documents').createSignedUrl('facture.pdf', { expiresIn: 3600 })

// Transformation d'image
garance.storage.from('avatars').getPublicUrl('user-123/photo.jpg', {
  transform: { width: 200, height: 200, format: 'webp' }
})
```

Métadonnées stockées dans PostgreSQL (schéma `garance_storage`). Fichiers binaires sur S3.

## 8. Dashboard Admin (Next.js)

### Pages

| Page | Fonctionnalités |
|---|---|
| Projets | Liste, création, settings, clés API |
| Table Editor | Vue tableur, CRUD inline, filtres, tri |
| SQL Editor | Requêtes SQL, historique, snippets |
| Schema | Visualisation, diff avec `garance.schema.ts`, migrations |
| Auth > Users | Liste utilisateurs, ban/unban, sessions |
| Auth > Providers | Configuration OAuth, clés, redirects |
| Storage > Buckets | Liste, permissions, quotas |
| Storage > Files | Navigateur, preview images, upload |
| API Docs | Documentation auto-générée (OpenAPI) |
| Settings | Clés API, webhooks, danger zone |
| Logs | Requêtes récentes, erreurs, latence |

### Modes

- **SaaS** (`dashboard.garance.io`) : multi-projets, billing, onboarding
- **Self-host** (`localhost:3000`) : mono/multi-projets, pas de billing

### Stack UI

Next.js (App Router), shadcn/ui + Tailwind, dark mode par défaut, Geist.

## 9. CLI (`garance`)

Binaire Go, distribué via Homebrew (`garancehq/tap/garance`), apt, et téléchargement direct.

| Commande | Description |
|---|---|
| `garance init` | Initialise un projet, crée `garance.schema.ts` |
| `garance dev` | Lance l'environnement local (Docker) |
| `garance db migrate` | Génère et applique les migrations |
| `garance db reset` | Reset la DB locale |
| `garance db seed` | Exécute le fichier de seed |
| `garance gen types` | Génère les types (`--lang ts\|dart\|swift\|kotlin`) |
| `garance login` | Connexion au compte garance.io |
| `garance link` | Lie le projet local à un projet SaaS |
| `garance deploy` | Déploie les migrations sur le projet SaaS |
| `garance status` | État du projet (local et remote) |

`garance dev` lance via Docker Compose : PostgreSQL, Query Engine, Auth, Storage, API Gateway, MinIO, Dashboard.

## 10. Structure du Monorepo

```
garancehq/garance/
├── engine/                      # Query Engine + Connection Pooler (Rust)
│   ├── crates/
│   │   ├── garance-engine/          # Introspection PG, génération API REST
│   │   ├── garance-pooler/          # Connection pooling
│   │   └── garance-codegen/         # Génération de types multi-langage
│   └── Cargo.toml
│
├── services/                    # Services applicatifs (Go)
│   ├── gateway/                     # API Gateway
│   ├── auth/                        # Auth service
│   ├── storage/                     # Storage service
│   └── go.work
│
├── cli/                         # CLI (Go)
│   └── cmd/garance/
│
├── dashboard/                   # Dashboard admin (Next.js)
│   ├── app/
│   ├── components/
│   └── package.json
│
├── sdks/                        # SDK clients
│   ├── typescript/                  # @garance/sdk
│   ├── dart/                        # garance_dart
│   └── ...
│
├── packages/                    # Packages partagés
│   ├── schema/                      # @garance/schema
│   └── shared/                      # Types et utilitaires partagés
│
├── deploy/                      # Déploiement
│   ├── docker-compose.yml           # Self-host production
│   ├── docker-compose.dev.yml       # Dev local
│   └── helm/                        # Chart Kubernetes (v1)
│
├── docs/                        # Documentation
└── .github/                     # CI/CD GitHub Actions
```

Build : Cargo workspace (Rust), Go workspaces (Go), pnpm workspaces (TypeScript). Pipelines CI indépendants par langage.

## 11. Self-host & Docker

### docker-compose.yml

```yaml
services:
  postgres:
    image: postgres:17-alpine
    volumes:
      - garance_data:/var/lib/postgresql/data
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    volumes:
      - garance_storage:/data

  engine:
    image: ghcr.io/garancehq/engine:latest
    depends_on: [postgres]
    environment:
      DATABASE_URL: postgresql://postgres:${DB_PASSWORD}@postgres:5432/garance

  gateway:
    image: ghcr.io/garancehq/gateway:latest
    ports:
      - "8080:8080"
    depends_on: [engine, auth, storage]

  auth:
    image: ghcr.io/garancehq/auth:latest
    depends_on: [postgres]

  storage:
    image: ghcr.io/garancehq/storage:latest
    depends_on: [minio]

  dashboard:
    image: ghcr.io/garancehq/dashboard:latest
    ports:
      - "3000:3000"
    depends_on: [gateway]

volumes:
  garance_data:
  garance_storage:
```

### Déploiement

```bash
curl -sSL https://garance.io/install.sh -o docker-compose.yml
echo "DB_PASSWORD=$(openssl rand -base64 32)" > .env
docker compose up -d
```

Images multi-arch (`linux/amd64` + `linux/arm64`) sur GitHub Container Registry (`ghcr.io/garancehq/*`).

## 12. Sécurité & Souveraineté

### Chiffrement

| Couche | Méthode |
|---|---|
| Transit | TLS 1.3 |
| DB at-rest | Chiffrement disque Scaleway (SaaS) |
| Storage at-rest | SSE-S3 |
| Secrets utilisateur | AES-256-GCM, clé par projet |
| Mots de passe | Argon2id |

### Isolation multi-tenant (SaaS)

- Un schéma PostgreSQL par projet
- `SET search_path = project_{id}` à chaque requête
- Buckets S3 préfixés par projet
- JWT contient `project_id`, vérifié systématiquement

### RGPD natif

| Fonctionnalité | Détail |
|---|---|
| Suppression de compte | `garance.auth.deleteUser()` + cascade optionnelle |
| Export des données | `garance.auth.exportUserData()` → JSON |
| Logs d'accès | Qui, quoi, quand — dashboard + API |
| Data residency | SaaS : France (Scaleway Paris). Self-host : au choix |
| DPA | Disponible pour clients SaaS |

### Audit trail

Log de toutes les opérations d'écriture dans `garance_audit.events`. Rétention : 30 jours (défaut), illimité (Enterprise).

### Certifications (roadmap)

| Certification | Cible |
|---|---|
| SOC 2 Type II | v1 |
| SecNumCloud | v2 |
| HDS | v2 |
| ISO 27001 | v2 |

## 13. Roadmap

### MVP (v0.1) — "Utilisable par un dev solo"

- Engine (Rust) : introspection PG, API REST, connection pooling
- Auth (Go) : email/password, magic link, OAuth (Google, GitHub, GitLab), JWT
- Storage (Go) : upload/download, buckets, signed URLs, S3-compatible
- Dashboard (Next.js) : table editor, SQL editor, users, buckets, files, settings
- CLI (Go) : init, dev, db migrate, gen types --lang ts, login, link, deploy
- SDK TypeScript : @garance/sdk
- Schema : @garance/schema
- Deploy : Docker Compose

### v0.2 — "Prêt pour la prod"

- Transformations d'images (libvips)
- Rate limiting configurable
- Webhooks
- Backups automatiques (SaaS)
- `garance gen types --lang dart`

### v1.0 — "Concurrent crédible"

- Realtime (Elixir) — subscriptions WebSocket
- FranceConnect
- SAML / SSO entreprise
- Passkeys / WebAuthn
- Helm Chart Kubernetes
- SDK Dart, Swift, Kotlin
- API GraphQL
- SOC 2

### v2.0 — "Plateforme complète"

- Edge Functions (runtime serverless multi-langage)
- AI (pgvector + LLMs européens)
- Multi-cloud EU (OVHcloud, Infomaniak)
- SecNumCloud / HDS
- SDK Python, Go, C#
- Marketplace d'extensions

### Ordre de construction du MVP

```
1. Engine (Rust)        ← fondation
2. Auth (Go)            ← sécurisation de l'API
3. Storage (Go)         ← parallélisable avec auth
4. CLI (Go)             ← orchestrateur
5. Schema (@garance)    ← s'appuie sur le engine
6. SDK TypeScript       ← consomme l'API
7. Dashboard (Next.js)  ← consomme les mêmes API
```

## 14. Identité & Distribution

| Élément | Valeur |
|---|---|
| Nom | Garance |
| Site / docs | garance.io |
| Dashboard SaaS | dashboard.garance.io |
| API projets | {projet}.garance.io |
| GitHub | garancehq/garance |
| npm | @garance/sdk, @garance/cli, @garance/schema |
| pub.dev | garance_dart |
| Homebrew | garancehq/tap/garance |
| Docker | ghcr.io/garancehq/* |
| Cloud | Scaleway (Paris) |

# Getting Started

This guide takes you from zero to a running Garance backend with a schema, data, and API calls.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (for the dev environment)
- [Node.js](https://nodejs.org/) 18+ (for the schema DSL and SDK)

## Install the CLI

```bash
brew install garancehq/tap/garance
```

Verify the installation:

```bash
garance version
```

## Create a project

```bash
garance init my-app
cd my-app
```

This creates the following files:

```
my-app/
├── garance.json         # Project configuration
├── garance.schema.ts    # Declarative schema
├── seeds/seed.sql       # Test data
├── migrations/          # SQL migrations
└── .env.local           # Local environment variables
```

## Define your schema

Open `garance.schema.ts` and define your tables:

```typescript
import { defineSchema, table, column, relation, bucket } from '@garance/schema'

export default defineSchema({
  users: table({
    id: column.uuid().primaryKey().default('gen_random_uuid()'),
    email: column.text().unique().notNull(),
    name: column.text().notNull(),
    created_at: column.timestamptz().default('now()'),
    posts: relation.hasMany('posts', 'author_id'),
  }),

  posts: table({
    id: column.uuid().primaryKey().default('gen_random_uuid()'),
    title: column.text().notNull(),
    content: column.text(),
    published: column.bool().default('false'),
    author_id: column.uuid().references('users.id'),
    created_at: column.timestamptz().default('now()'),
    author: relation.belongsTo('users', 'author_id'),
    access: {
      read: (ctx) => ctx.where({ published: true }).or(ctx.isOwner('author_id')),
      write: (ctx) => ctx.isOwner('author_id'),
    },
  }),

  storage: {
    avatars: bucket({
      maxFileSize: '5mb',
      allowedMimeTypes: ['image/jpeg', 'image/png', 'image/webp'],
      access: {
        read: 'public',
        write: (ctx) => ctx.isAuthenticated(),
      },
    }),
  },
})
```

## Start the dev environment

```bash
garance dev
```

This starts all services via Docker Compose:

| Service    | URL                     |
|------------|-------------------------|
| Gateway    | http://localhost:8080    |
| Engine     | http://localhost:4000    |
| Auth       | http://localhost:4001    |
| Storage    | http://localhost:4002    |
| MinIO      | http://localhost:9001    |
| PostgreSQL | localhost:5432           |

## Run your first migration

Compile the schema and apply it to the database:

```bash
garance db migrate
```

The CLI will:

1. Compile `garance.schema.ts` to JSON
2. Send the schema to the Engine for diff calculation
3. Show the SQL migration preview
4. Save the migration file to `migrations/`
5. Apply the migration

## Test with curl

### Create a user

```bash
curl -X POST http://localhost:8080/auth/v1/signup \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "password": "supersecret"}'
```

Response:

```json
{
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "alice@example.com",
    "email_verified": false,
    "role": "user",
    "created_at": "2026-03-26T10:00:00Z",
    "updated_at": "2026-03-26T10:00:00Z"
  },
  "token_pair": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "dGhpcyBpcyBhIHJlZnJl...",
    "expires_in": 3600,
    "token_type": "Bearer"
  }
}
```

### Insert a row

```bash
curl -X POST http://localhost:8080/api/v1/posts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <access_token>" \
  -d '{"title": "Hello Garance", "content": "My first post", "published": true}'
```

### Query data

```bash
curl "http://localhost:8080/api/v1/posts?published=eq.true&order=created_at.desc&limit=10"
```

## Use the TypeScript SDK

Install the SDK in your frontend or backend project:

```bash
npm install @garance/sdk
```

```typescript
import { createClient } from '@garance/sdk'

const garance = createClient({ url: 'http://localhost:8080' })

// Sign up
await garance.auth.signUp({ email: 'alice@example.com', password: 'supersecret' })

// Insert data
const { data: post } = await garance.from('posts').insert({
  title: 'Hello Garance',
  content: 'My first post',
  published: true,
})

// Query data
const { data: posts } = await garance
  .from('posts')
  .eq('published', true)
  .order('created_at', 'desc')
  .limit(10)
  .execute()

// Upload a file
const file = new File(['avatar'], 'avatar.png', { type: 'image/png' })
await garance.storage.from('avatars').upload('alice/photo.png', file)
```

## Generate client types

```bash
garance gen types --lang ts
```

This generates TypeScript types from your database schema at `types/garance.ts`.

## Open the Dashboard

The admin dashboard is available at **http://localhost:3000** when running `garance dev`. It provides:

- **Table editor** -- browse and edit data with an inline spreadsheet UI
- **SQL editor** -- run arbitrary SQL queries
- **User management** -- view, search, and manage user accounts
- **Storage browser** -- browse buckets and files
- **Settings** -- project configuration, API keys, environment

## Next steps

- [Schema DSL](/schema) -- full reference for `defineSchema`, column types, relations, and access rules
- [Authentication](/auth) -- email/password, OAuth, magic links, JWT
- [Storage](/storage) -- buckets, uploads, signed URLs
- [TypeScript SDK](/sdk) -- complete client API
- [CLI Reference](/cli) -- all available commands
- [REST API](/api) -- HTTP endpoints reference
- [Self-Hosting](/self-host) -- deploy Garance in production with Docker Compose

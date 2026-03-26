# CLI Reference

The Garance CLI manages project setup, local development, database migrations, and code generation.

## Installation

```bash
brew install garancehq/tap/garance
```

## Commands

### `garance init [name]`

Create a new Garance project.

```bash
garance init my-app
```

If `name` is provided, creates a subdirectory. Otherwise, initializes the current directory.

**Generated files:**

| File                 | Description                    |
|----------------------|--------------------------------|
| `garance.json`       | Project configuration          |
| `garance.schema.ts`  | Declarative schema file        |
| `seeds/seed.sql`     | Test seed data                 |
| `migrations/`        | SQL migration directory        |
| `.env.local`         | Local environment variables    |

---

### `garance dev`

Start the local development environment via Docker Compose.

```bash
garance dev
```

Starts PostgreSQL, MinIO, Engine, Auth, Storage, and Gateway.

| Service    | URL                     |
|------------|-------------------------|
| Gateway    | http://localhost:8080    |
| Engine     | http://localhost:4000    |
| Auth       | http://localhost:4001    |
| Storage    | http://localhost:4002    |
| MinIO      | http://localhost:9001    |
| PostgreSQL | localhost:5432           |

#### `garance dev stop`

Stop all running services.

```bash
garance dev stop
```

#### `garance dev status`

Show the state of all services.

```bash
garance dev status
```

#### `garance dev logs [service]`

Show service logs.

```bash
garance dev logs           # All services
garance dev logs engine    # Engine only
garance dev logs -f        # Follow mode (real-time)
```

**Flags:**

| Flag          | Description                |
|---------------|----------------------------|
| `-f, --follow` | Follow logs in real-time  |

---

### `garance db migrate`

Compile the schema and apply a migration to the database.

```bash
garance db migrate
```

**Pipeline:**

1. Compiles `garance.schema.ts` to `garance.schema.json`
2. Sends the schema to the Engine for diff
3. Displays the SQL migration preview and operations
4. Prompts for confirmation if destructive
5. Saves the migration to `migrations/<timestamp>_<description>.sql`
6. Applies the migration

**Flags:**

| Flag    | Description                                  |
|---------|----------------------------------------------|
| `--yes` | Skip confirmation prompt for destructive migrations |

**Environment:**

| Variable     | Description                  | Default                  |
|-------------|------------------------------|--------------------------|
| `ENGINE_URL` | Engine HTTP endpoint          | `http://localhost:4000`  |

---

### `garance db seed`

Run the seed file against the database.

```bash
garance db seed
```

Executes `seeds/seed.sql` against the connected database.

---

### `garance db reset`

Drop and recreate the database, then re-apply all migrations.

```bash
garance db reset
```

::: warning
This permanently deletes all data. Use only in development.
:::

---

### `garance gen types`

Generate typed client code from the database schema.

```bash
garance gen types --lang ts
garance gen types --lang dart --output lib/garance.dart
```

**Flags:**

| Flag               | Description                                    | Default                  |
|--------------------|------------------------------------------------|--------------------------|
| `-l, --lang`       | Target language (`ts`, `dart`, `swift`, `kotlin`) | `ts`                    |
| `-o, --output`     | Output file path                               | `types/garance.<ext>`    |

**Environment:**

| Variable     | Description                  | Default                  |
|-------------|------------------------------|--------------------------|
| `ENGINE_URL` | Engine HTTP endpoint          | `http://localhost:4000`  |

---

### `garance status`

Display the project status and check service health.

```bash
garance status
```

Output:

```
Projet : my-app (v0.1.0)
Engine : latest

Services locaux :
  Gateway      ✓ en ligne
  Engine       ✓ en ligne
  Auth         ✓ en ligne
  Storage      ✗ arrêté
```

---

### `garance version`

Print the CLI version and git commit.

```bash
garance version
# garance 0.1.0 (a1b2c3d)
```

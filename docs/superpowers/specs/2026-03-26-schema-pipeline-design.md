# Schema Pipeline — Design Spec

> Design document — March 26, 2026

## Goal

Implement the schema pipeline: compile `garance.schema.ts` to JSON, diff it against the current PostgreSQL schema, generate SQL migrations, and apply them. This is Garance's core differentiator — declarative schema in TypeScript, automatic migrations.

## Flow

```
garance db migrate
  │
  ├─ CLI: compile garance.schema.ts → garance.schema.json (via npx tsx)
  │
  ├─ CLI: POST /api/v1/_migrate/preview { schema JSON }
  │       Engine: introspect PG → diff(desired, current) → SQL statements
  │
  ├─ CLI: display preview, confirm if destructive
  ├─ CLI: save migration file to migrations/
  │
  ├─ CLI: POST /api/v1/_migrate/apply { sql, filename }
  │       Engine: BEGIN → execute SQL → record in tracking table → COMMIT → reload schema
  │
  └─ Done
```

## 1. Diff Engine (Rust)

New module in `garance-engine`: compares a `GaranceSchema` (desired, from JSON) with a `Schema` (current, from PG introspection) and produces ordered SQL statements.

### Operations Detected

| Change | SQL Generated |
|---|---|
| New table | `CREATE TABLE` with all columns, defaults, constraints |
| Dropped table | `DROP TABLE` |
| New column | `ALTER TABLE ADD COLUMN` |
| Dropped column | `ALTER TABLE DROP COLUMN` |
| Type changed | `ALTER TABLE ALTER COLUMN TYPE ... USING` |
| Nullable → not null | `ALTER TABLE ALTER COLUMN SET NOT NULL` |
| Not null → nullable | `ALTER TABLE ALTER COLUMN DROP NOT NULL` |
| Default added | `ALTER TABLE ALTER COLUMN SET DEFAULT` |
| Default removed | `ALTER TABLE ALTER COLUMN DROP DEFAULT` |
| New foreign key | `ALTER TABLE ADD CONSTRAINT ... FOREIGN KEY` |
| Dropped foreign key | `ALTER TABLE DROP CONSTRAINT` |
| New index | `CREATE INDEX` |
| Dropped index | `DROP INDEX` |
| New unique constraint | `ALTER TABLE ADD CONSTRAINT ... UNIQUE` |
| Dropped unique constraint | `ALTER TABLE DROP CONSTRAINT` |

**Not supported (MVP):** table/column renaming. A rename is detected as a drop + create. If the developer needs a rename without data loss, they write a manual SQL migration file.

### Type Normalization

The diff engine must convert between `GaranceColumn.col_type` (string from JSON, e.g., `"text"`, `"varchar"`) and `PgType` (enum from introspection). A canonical normalization function ensures no false positives:

```
normalize("text") = normalize("varchar") = normalize("character varying") = "text"
normalize("int4") = normalize("integer") = normalize("int") = "int4"
normalize("int8") = normalize("bigint") = "int8"
normalize("float8") = normalize("double precision") = "float8"
normalize("bool") = normalize("boolean") = "bool"
normalize("timestamp") = normalize("timestamp without time zone") = "timestamp"
normalize("timestamptz") = normalize("timestamp with time zone") = "timestamptz"
normalize(other) = other  // pass through for custom types
```

Both sides are normalized before comparison. The diff engine uses this same function for `GaranceColumn.col_type` and `PgType.to_canonical_string()`.

`serial` / `bigserial` are treated specially: in PG they are `int4` / `int8` with a `nextval()` default. The diff compares the underlying type, not the alias.

### Default Expression Normalization

PostgreSQL returns different textual representations for the same default value. The diff normalizes before comparing:

```
normalize_default("now()") = normalize_default("CURRENT_TIMESTAMP") = "now()"
normalize_default("gen_random_uuid()") = "gen_random_uuid()"
normalize_default("'foo'::text") = "'foo'::text"  // leave casts as-is
normalize_default("true") = normalize_default("'t'::boolean") = "true"
normalize_default("false") = normalize_default("'f'::boolean") = "false"
```

If normalization doesn't match a known pattern, the raw strings are compared directly. This may produce false positives for exotic defaults — acceptable for MVP.

### Scope: `public` Schema Only

The diff engine only considers tables in the `public` schema (or `project_{id}` in multi-tenant mode). Tables in `garance_auth`, `garance_storage`, `garance_platform`, `pg_catalog`, and `information_schema` are completely invisible to the diff. The introspection function already scopes to `public` — no change needed there.

### Constraint Names

**Required model change:** `ForeignKey` in `types.rs` must be extended with a `constraint_name: String` field. Without the constraint name, `DROP CONSTRAINT` cannot be generated. The introspection already reads `tc.constraint_name` from `information_schema.table_constraints` — it just needs to be stored.

Similarly, unique constraints need their names stored for `DROP CONSTRAINT`. The `Index` type already has a `name` field which covers unique indexes. For unique constraints that aren't indexes (rare), the constraint name is inferred from the index name.

### Self-Referential Foreign Keys

When creating a table with a FK that references itself (e.g., `nodes.parent_id → nodes.id`), the FK must be added as a separate `ALTER TABLE ADD CONSTRAINT` after the `CREATE TABLE`, not inline in the `CREATE TABLE` statement. The diff engine treats ALL foreign keys as deferred — they are always added in phase 5, never in the `CREATE TABLE`.

### Statement Ordering

Critical for foreign key correctness:

1. Create new tables (without FK — all FK deferred to step 7)
2. Add columns
3. Drop NOT NULL (where changing nullable→not null requires type change first)
4. Alter column types (`ALTER COLUMN TYPE ... USING`)
5. Set/drop defaults
6. Set NOT NULL (after type changes are done)
7. Add foreign keys
8. Add unique constraints
9. Create indexes
10. Drop indexes
11. Drop unique constraints
12. Drop foreign keys
13. Drop columns
14. Drop tables

### Destructive Detection

`has_destructive` is `true` if the diff contains any `DROP TABLE` or `DROP COLUMN`. The CLI shows a warning and asks for confirmation before applying.

## 2. Engine Endpoints

### `POST /api/v1/_migrate/preview`

Receives the desired schema JSON, diffs with current PG, returns SQL without applying.

**Request:**
```json
{
  "schema": { "version": 1, "tables": { ... }, "storage": { ... } }
}
```

**Response:**
```json
{
  "statements": [
    "CREATE TABLE posts (\n  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),\n  title text NOT NULL\n);",
    "ALTER TABLE users ADD COLUMN bio text;"
  ],
  "summary": {
    "tables_created": 1,
    "tables_dropped": 0,
    "columns_added": 1,
    "columns_dropped": 0,
    "columns_modified": 0,
    "indexes_created": 0,
    "indexes_dropped": 0,
    "foreign_keys_added": 0,
    "foreign_keys_dropped": 0
  },
  "has_destructive": false
}
```

If no changes detected, `statements` is empty and summary is all zeros.

**Validation:** If the schema JSON is malformed or contains references to non-existent tables (e.g., a FK referencing a table not in the schema), the endpoint returns 400 with a structured error message. The diff engine validates referential integrity before generating SQL.

**Note on `/apply`:** The CLI may modify the SQL file between preview and apply. The Engine executes whatever SQL is sent — it does not verify that it matches the last preview. This is by design: the developer may want to hand-edit the migration before applying. The checksum in the tracking table records what was actually applied.

### `POST /api/v1/_migrate/apply`

Applies SQL migration, records in tracking table, reloads schema.

**Request:**
```json
{
  "sql": "CREATE TABLE posts (...);\nALTER TABLE users ADD COLUMN bio text;",
  "filename": "20260326120000_add_posts_and_bio.sql"
}
```

**Response:**
```json
{
  "applied": true,
  "filename": "20260326120000_add_posts_and_bio.sql",
  "tables_after": 3
}
```

Execution is wrapped in a transaction. If any statement fails, the entire migration is rolled back — nothing applied, nothing recorded.

If the filename already exists in the tracking table, returns 409 CONFLICT.

## 3. Migration Tracking Table

```sql
CREATE SCHEMA IF NOT EXISTS garance_platform;

CREATE TABLE IF NOT EXISTS garance_platform.migrations (
    id SERIAL PRIMARY KEY,
    filename TEXT UNIQUE NOT NULL,
    checksum TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- Created automatically by the Engine on first `_migrate/apply` call (idempotent)
- `checksum` is SHA-256 of the SQL content
- Used to prevent double-application and detect tampered migration files

## 4. Migration File Format

Files stored in `migrations/` at the project root:

```
migrations/
├── 20260326120000_initial.sql
├── 20260326143000_add_posts.sql
└── 20260326150000_add_bio_column.sql
```

Naming: `{YYYYMMDDHHMMSS}_{description}.sql`

The description is auto-generated from the diff summary (e.g., `add_posts`, `add_bio_to_users`, `drop_legacy_table`). The developer can rename the file before committing — only the content matters for tracking.

Content is plain SQL — the exact statements from the diff, separated by newlines.

## 5. CLI Orchestration

`garance db migrate` flow:

1. **Compile**: write a temp script that imports `garance.schema.ts` and calls `compile()`, execute via `npx tsx`, capture JSON output to `garance.schema.json`
2. **Preview**: `POST /api/v1/_migrate/preview` with the JSON
3. **Confirm**: if `has_destructive` is true, display warning and ask `y/N`. Skip with `--yes` flag.
4. **Save**: write SQL to `migrations/{timestamp}_{description}.sql`
5. **Apply**: `POST /api/v1/_migrate/apply` with SQL and filename
6. **Done**: display summary

**If no changes detected** (empty statements), the CLI says "Schema is up to date" and exits.

**Prerequisites**: Node.js + `@garance/schema` installed in the project. `garance init` creates a `package.json` with `@garance/schema` as dependency and tells the user to run `npm install`.

## 6. Changes to @garance/schema

### Column `renamedFrom` method

Added to the builder API for forward compatibility. Produces a `renamed_from` field in the JSON. Ignored by the diff engine at MVP — treated as drop + create.

```typescript
full_name: column.text().notNull().renamedFrom('name')
```

JSON output:
```json
{ "type": "text", "nullable": false, "renamed_from": "name" }
```

### `garance init` template update

The project template now includes `package.json`:
```json
{
  "private": true,
  "dependencies": {
    "@garance/schema": "^0.1.0"
  }
}
```

## 7. Implementation Scope

| Component | Changes |
|---|---|
| Engine: new module `diff/` | Diff engine: compare GaranceSchema vs Schema, produce SQL |
| Engine: `api/routes.rs` | 2 new handlers: `migrate_preview`, `migrate_apply` |
| Engine: `api/mod.rs` | 2 new routes |
| Engine: proto + gRPC | 2 new RPCs: `MigratePreview`, `MigrateApply` |
| @garance/schema | `renamedFrom` method on ColumnBuilder, `renamed_from` in JSON output |
| CLI: `cmd/db.go` | Rewrite `garance db migrate` to use the pipeline |
| CLI: `internal/project/templates.go` | Add `package.json` to init template |
| Gateway: `proxy/engine.go` | 2 new proxy routes |

## 8. Testing

| Test | Component | Verifies |
|---|---|---|
| `test_diff_create_table` | Engine diff | New table → CREATE TABLE |
| `test_diff_drop_table` | Engine diff | Removed table → DROP TABLE |
| `test_diff_add_column` | Engine diff | New column → ALTER TABLE ADD COLUMN |
| `test_diff_drop_column` | Engine diff | Removed column → ALTER TABLE DROP COLUMN |
| `test_diff_change_type` | Engine diff | Type change → ALTER COLUMN TYPE |
| `test_diff_change_nullable` | Engine diff | Nullable change → SET/DROP NOT NULL |
| `test_diff_change_default` | Engine diff | Default change → SET/DROP DEFAULT |
| `test_diff_add_foreign_key` | Engine diff | New FK → ADD CONSTRAINT |
| `test_diff_drop_foreign_key` | Engine diff | Removed FK → DROP CONSTRAINT |
| `test_diff_add_index` | Engine diff | New index → CREATE INDEX |
| `test_diff_no_changes` | Engine diff | Identical schemas → empty SQL |
| `test_diff_statement_ordering` | Engine diff | FK after tables, drops in reverse order |
| `test_diff_destructive_flag` | Engine diff | DROP TABLE → has_destructive true |
| `test_migrate_preview_endpoint` | Engine API | Preview returns correct SQL |
| `test_migrate_apply_endpoint` | Engine API | Apply executes SQL, records tracking |
| `test_migrate_tracking` | Engine API | Same migration rejected twice (409) |
| `test_diff_self_referential_fk` | Engine diff | FK on same table → deferred to ALTER TABLE |
| `test_diff_type_normalization` | Engine diff | `varchar` vs `text` → no false positive |
| `test_diff_default_normalization` | Engine diff | `now()` vs `CURRENT_TIMESTAMP` → no false positive |
| `test_diff_ignores_system_tables` | Engine diff | garance_platform tables not in diff |
| `test_migrate_apply_rollback_on_error` | Engine API | Invalid SQL → full rollback, nothing recorded |

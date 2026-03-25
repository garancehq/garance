# Engine V2 — New Endpoints Design

> Design document — March 25, 2026

## Goal

Add 5 new endpoints to the Garance Engine to make the dashboard functional and improve the developer experience: table listing, full schema introspection, scoped SQL execution, and hot-reload of the schema without restart.

## New Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/api/v1/_tables` | GET | List introspected tables with metadata |
| `/api/v1/_schema` | GET | Full introspected schema (tables, columns, types, FK, indexes) |
| `/api/v1/_schema/{table}` | GET | Schema for a single table |
| `/api/v1/rpc/query` | POST | Execute scoped SQL |
| `/api/v1/_reload` | POST | Re-introspect PG schema without restart |

## 1. Table Listing (`_tables`)

Returns a summary of all introspected tables.

**Response:**
```json
[
  { "name": "todos", "columns": 4, "primary_key": ["id"], "row_count": 3 },
  { "name": "users", "columns": 5, "primary_key": ["id"], "row_count": 12 }
]
```

- `primary_key` is `string[]` (supports composite PKs). `null` if no PK defined.
- `row_count` is obtained via `pg_stat_user_tables.n_live_tup` (approximate, no full table scan). Returns `0` for tables that haven't been autovacuumed yet. Returns `null` only if the table is absent from `pg_stat_user_tables` (shouldn't happen for user tables).
- `_tables` requires a PG query to fetch `n_live_tup` — this is the only metadata endpoint that hits the database.

**Reserved names:** tables named `_tables`, `_schema`, `_reload`, or `rpc` will conflict with Engine endpoints. The Engine should reject these names if they appear in the introspected schema (log a warning, skip the table).

## 2. Schema Introspection (`_schema`)

Returns the full `Schema` struct already in memory (from the introspection at startup). No additional PG query needed — just serialize `AppState.schema` to JSON.

**`GET /api/v1/_schema` response:** the full `Schema` object (all tables).

**`GET /api/v1/_schema/{table}` response:** a single `Table` object. Returns 404 (`{"error": {"code": "NOT_FOUND", ...}}`) if the table doesn't exist — same error format as all other endpoints.

The response format matches the `Schema` / `Table` types already defined in `schema/types.rs` (with serde Serialize).

## 3. SQL Execution (`rpc/query`)

Executes arbitrary SQL scoped to the project's schema.

**Request:**
```json
{
  "sql": "SELECT * FROM todos WHERE completed = false"
}
```

**Response:**
```json
{
  "columns": ["id", "title", "completed"],
  "rows": [
    { "id": "abc-123", "title": "Build MVP", "completed": false }
  ],
  "row_count": 1,
  "duration_ms": 12
}
```

**Error response (standard format):**
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "references to internal schema 'garance_auth' are not allowed",
    "status": 400
  }
}
```

### Security

Three protections applied before execution:

1. **Search path forced** — `SET LOCAL search_path TO public` (or `project_{id}` in multi-tenant mode) at the start of each transaction. `SET LOCAL` scopes to the transaction, not the session — critical for connection pool correctness. Unqualified table names resolve to the project schema only.

2. **Internal schema blocking** — static scan of the SQL string before execution. Immediate rejection (400 VALIDATION_ERROR) if the SQL contains a reference to:
   - `garance_auth.`
   - `garance_storage.`
   - `garance_platform.`
   - `garance_audit.`
   - `information_schema.`

   Case-insensitive string matching. Not a full SQL parser — protects against mistakes and casual abuse, not determined attackers.

   **Important:** In production, the Engine's PG connection must use a restricted PostgreSQL role that only has access to the project schema. This is a **prerequisite**, not an optional hardening step. The string scan is a first line of defense; the PG role is the real security boundary.

3. **Read-only by default** — the transaction uses `SET TRANSACTION READ ONLY`. To allow writes (INSERT, UPDATE, DELETE, DDL), the client must send the header `X-Garance-SQL-Mode: readwrite`. The dashboard SQL Editor sends this header automatically.

### Execution flow

```
POST /api/v1/rpc/query { "sql": "..." }
  │
  ├─ Validate: reject empty SQL, reject multi-statement SQL (containing ';')
  ├─ Validate: reject SQL exceeding 64KB (DoS protection)
  ├─ Scan for blocked schema references → 400 if found
  │
  ├─ Acquire connection from pool
  ├─ BEGIN
  ├─ SET LOCAL search_path TO public
  ├─ SET TRANSACTION READ ONLY  (unless X-Garance-SQL-Mode: readwrite)
  │
  ├─ Execute SQL
  │
  ├─ Serialize results using existing row_to_json
  ├─ COMMIT
  │
  └─ Return { columns, rows, row_count, duration_ms }
```

For non-SELECT statements (INSERT, UPDATE, DELETE, DDL) in readwrite mode, the response includes `row_count` (affected rows) and empty `rows`/`columns`.

**Note:** After DDL in readwrite mode (CREATE TABLE, ALTER TABLE, etc.), the in-memory schema is stale. The client should call `POST /api/v1/_reload` to refresh it. The SQL endpoint does NOT auto-reload.

## 4. Schema Hot-Reload (`_reload`)

Re-runs `introspect(&client, "public")` and replaces the in-memory schema.

**Response:**
```json
{
  "tables": 3,
  "reloaded_at": "2026-03-25T16:00:00Z"
}
```

**Mechanism:**
- Acquires a connection from the pool
- Calls `introspect()` (same function as startup)
- **If introspect() fails:** returns a 500 error, the in-memory schema remains unchanged
- **If introspect() succeeds:** replaces the schema via `schema.write().await`
- No downtime — in-flight requests use the old schema, new requests see the updated one

**Error safety pattern:**
```rust
// Introspect FIRST, before acquiring the write lock
let new_schema = introspect(&client, "public").await?; // error → early return, schema untouched
let mut schema = state.schema.write().await;
*schema = new_schema; // atomic replacement
```

**CLI integration:**
- `garance db migrate` calls `POST http://localhost:4000/api/v1/_reload` after applying migrations
- If the Engine is unreachable, the CLI ignores the error silently

## 5. Implementation Scope

All changes are in the existing `garance-engine` crate:

- **`api/routes.rs`** — 5 new handler functions
- **`api/mod.rs`** — 5 new routes registered
- **`grpc/server.rs`** — corresponding gRPC methods (ListTables, GetSchema, ExecuteSQL, ReloadSchema)
- **Proto** — `proto/engine/v1/engine.proto` updated with new RPCs

No new modules, no new crates. The existing `introspect`, `row_to_json`, `AppState`, and `Schema` types are reused as-is.

## 6. Testing

| Test | Type | What it verifies |
|---|---|---|
| `test_list_tables` | Integration | Returns table names, column counts, PK |
| `test_get_schema` | Integration | Returns full schema matching introspected data |
| `test_get_schema_single_table` | Integration | Returns one table, 404 for unknown |
| `test_sql_select` | Integration | Executes SELECT, returns rows + columns |
| `test_sql_insert_readonly` | Integration | Rejects INSERT in read-only mode |
| `test_sql_insert_readwrite` | Integration | Allows INSERT with readwrite header |
| `test_sql_blocked_schema` | Integration | Rejects SQL referencing garance_auth |
| `test_sql_empty` | Integration | Rejects empty SQL with 400 |
| `test_sql_multi_statement` | Integration | Rejects SQL with semicolons |
| `test_reload_schema` | Integration | New table visible after reload |
| `test_reload_error_preserves_schema` | Integration | Failed reload keeps old schema intact |

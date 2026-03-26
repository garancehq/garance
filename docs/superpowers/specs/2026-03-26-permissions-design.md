# Permissions (Row Level Security) — Design Spec

> Design document — March 26, 2026

## Goal

Enforce access control on database tables using PostgreSQL Row Level Security. Access rules defined in `garance.schema.ts` are compiled to RLS policies during migrations. All data access (REST API, raw SQL, Realtime) is governed by these policies.

## Security Model: Deny by Default

RLS is enabled on **every user table**, regardless of whether access rules are defined. A table without access rules has no policies, which means PostgreSQL blocks all access for non-owner roles. The developer must explicitly opt-in to access.

This is the opposite of the initial design (opt-in). Rationale: a forgotten access rule should result in blocked access, not exposed data.

For tables that should be publicly accessible:
```typescript
access: {
  read: "public",       // anyone can read
  write: "authenticated" // any logged-in user can write
}
```

## 1. Access Rule → RLS Policy Translation

### Condition Types

| DSL | JSON | SQL (USING clause) |
|---|---|---|
| `"public"` | `"public"` | `true` |
| `"authenticated"` | `"authenticated"` | `current_setting('request.user_id', true) IS NOT NULL` |
| `ctx.isOwner('col')` | `{ "type": "isOwner", "column": "col" }` | `"col"::text = current_setting('request.user_id', true)` |
| `ctx.where({ col: val })` | `{ "type": "where", "filters": { "col": val } }` | `"col" = {quote_literal(val)}` |
| `ctx.isAuthenticated()` | `{ "type": "isAuthenticated" }` | `current_setting('request.user_id', true) IS NOT NULL` |

### Performance Optimization

User identity is stored in a dedicated session variable (not parsed from JSON on every row):

```sql
SET LOCAL request.user_id TO 'abc-123';
SET LOCAL request.user_role TO 'user';
```

Policies use `current_setting('request.user_id', true)` — simple string comparison, no JSON parsing per row.

### Multiple Conditions = OR

Multiple conditions in a single rule are combined with OR:

```typescript
read: (ctx) => ctx.where({ published: true }).or(ctx.isOwner('author_id'))
```

```sql
USING (("published" = true) OR ("author_id"::text = current_setting('request.user_id', true)))
```

### Full Example

```typescript
access: {
  read: (ctx) => ctx.where({ published: true }).or(ctx.isOwner('author_id')),
  write: (ctx) => ctx.isOwner('author_id'),
  delete: (ctx) => ctx.isOwner('author_id'),
}
```

Generated SQL:

```sql
ALTER TABLE "posts" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "posts" FORCE ROW LEVEL SECURITY;

-- SELECT: published OR owner
CREATE POLICY "garance_select_posts" ON "posts"
  FOR SELECT USING (
    ("published" = true)
    OR ("author_id"::text = current_setting('request.user_id', true))
  );

-- INSERT: must be owner of the row being created
CREATE POLICY "garance_insert_posts" ON "posts"
  FOR INSERT WITH CHECK (
    "author_id"::text = current_setting('request.user_id', true)
  );

-- UPDATE: can only target own rows, result must also be own row
CREATE POLICY "garance_update_posts" ON "posts"
  FOR UPDATE
  USING ("author_id"::text = current_setting('request.user_id', true))
  WITH CHECK ("author_id"::text = current_setting('request.user_id', true));

-- DELETE: explicit, does NOT inherit from write
CREATE POLICY "garance_delete_posts" ON "posts"
  FOR DELETE USING (
    "author_id"::text = current_setting('request.user_id', true)
  );
```

### Key Rules

- **UPDATE has both USING and WITH CHECK** — prevents transferring row ownership via UPDATE
- **DELETE does NOT inherit from write** — must be declared explicitly. If not declared, no DELETE policy exists → deletes are blocked
- **Filter values are escaped** — `where({ col: val })` uses `quote_literal()` for string values, type-safe literals for booleans/numbers. Column names are validated against the table schema.
- **`FORCE ROW LEVEL SECURITY`** on all tables — ensures policies apply even if query runs as table owner (e.g., SET LOCAL forgotten)

## 2. PostgreSQL Roles

```sql
-- Created by Engine on startup (idempotent)
DO $$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'garance_anon') THEN
    CREATE ROLE garance_anon NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'garance_authenticated') THEN
    CREATE ROLE garance_authenticated NOLOGIN;
  END IF;
END $$;

GRANT USAGE ON SCHEMA public TO garance_anon, garance_authenticated;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO garance_anon;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO garance_authenticated;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO garance_anon;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO garance_authenticated;
```

**Important:** The Engine's database connection role must NOT be a SUPERUSER (superusers bypass all RLS). Use a regular owner role with `CREATEROLE` privilege to manage the session roles.

## 3. Session Setup (Engine)

Before every data query, the Engine wraps in a transaction with role setup:

```sql
BEGIN;
SET LOCAL role TO 'garance_authenticated';  -- or 'garance_anon' if no JWT
SET LOCAL request.user_id TO 'abc-123';     -- from JWT claims
SET LOCAL request.user_role TO 'user';      -- from JWT claims
-- execute query --
COMMIT;
```

**Every handler** (`list_rows`, `get_row`, `insert_row`, `update_row`, `delete_row`, `execute_sql`) must go through this wrapper. No query executes without the SET LOCAL.

**Error mapping:**
- RLS violation on INSERT/UPDATE/DELETE → PostgreSQL error code `42501` (insufficient_privilege) → Engine returns `403 PERMISSION_DENIED`
- RLS on SELECT → silently returns 0 rows (no error)

## 4. `rpc/query` Security Hardening

The existing SQL guard is extended to block privilege escalation:

```
Blocked SQL patterns (in addition to existing schema blocking):
- SET ROLE / SET LOCAL ROLE / RESET ROLE
- SET SESSION / SET LOCAL request.*
- GRANT / REVOKE
- CREATE ROLE / ALTER ROLE / DROP ROLE
- CREATE POLICY / ALTER POLICY / DROP POLICY
```

These are blocked by string matching in the SQL guard (same approach as schema blocking). The RLS provides the real security boundary — the SQL guard is defense in depth.

Additionally, restrict access to system catalogs:

```sql
REVOKE ALL ON SCHEMA pg_catalog FROM garance_anon, garance_authenticated;
GRANT USAGE ON SCHEMA pg_catalog TO garance_anon, garance_authenticated;
-- Only allow specific safe views
GRANT SELECT ON pg_catalog.pg_type TO garance_anon, garance_authenticated;
```

## 5. Realtime Filtering

PostgreSQL triggers (used for NOTIFY) execute as the table owner — RLS policies do NOT apply inside triggers. This means all changes are notified regardless of permissions.

**Solution:** The Realtime service (Elixir) must filter notifications before pushing to clients:

1. Each WebSocket client has a `user_id` (from JWT at connection time)
2. When a NOTIFY arrives, the Dispatcher checks each subscriber's `user_id` against the payload using the same logic as the access rules
3. If the subscriber would not be able to see the row via the REST API, the notification is suppressed

For MVP: the Realtime service evaluates `isOwner` checks locally (compare `payload.new.{column}` with the subscriber's `user_id`). For `where` conditions, it evaluates the filter against the payload data. This mirrors the RLS logic in application code.

## 6. Diff Engine Changes

The diff engine is extended to generate RLS statements during migrations:

**For every user table (with or without access rules):**
```sql
ALTER TABLE "{table}" ENABLE ROW LEVEL SECURITY;
ALTER TABLE "{table}" FORCE ROW LEVEL SECURITY;
```

**For tables with access rules — generate policies:**
- `garance_select_{table}` (from `read`)
- `garance_insert_{table}` (from `write`)
- `garance_update_{table}` (from `write`, with USING + WITH CHECK)
- `garance_delete_{table}` (from `delete`, explicit only)

**For changed access rules:**
- `DROP POLICY IF EXISTS` old policies
- `CREATE POLICY` new policies
- Wrapped in a single transaction (no window of no-policies)

**For removed access rules:**
- Drop `garance_*` policies
- RLS stays enabled (deny by default)

**Policy naming:** `garance_{operation}_{table}` (e.g., `garance_select_posts`).

**Introspection:** the diff reads existing policies from `pg_policies` system view.

## 7. Implementation Scope

| Component | Changes |
|---|---|
| Engine: new `schema/rls.rs` | RLS policy SQL generation from access rules |
| Engine: new `schema/roles.rs` | PG role creation (garance_anon, garance_authenticated) |
| Engine: `api/routes.rs` | Wrap all handlers in SET LOCAL role transaction |
| Engine: `api/sql_guard.rs` | Block SET ROLE, GRANT, CREATE ROLE, etc. |
| Engine: `main.rs` | Create roles on startup |
| Engine: `diff/diff.rs` | Generate ENABLE RLS + policies in migrations |
| Realtime: `dispatcher.ex` | Filter notifications by subscriber permissions |
| Realtime: `changes_channel.ex` | Store user_id from JWT at connection time |

## 8. Testing

| Test | Component | Verifies |
|---|---|---|
| `test_rls_policy_isowner` | rls.rs | isOwner → correct USING clause |
| `test_rls_policy_where` | rls.rs | where filter → escaped value |
| `test_rls_policy_public` | rls.rs | "public" → USING (true) |
| `test_rls_policy_authenticated` | rls.rs | isAuthenticated → user_id IS NOT NULL |
| `test_rls_policy_combined_or` | rls.rs | Multiple conditions → OR combined |
| `test_rls_update_has_with_check` | rls.rs | UPDATE policy has USING + WITH CHECK |
| `test_rls_delete_not_inherited` | rls.rs | No delete rule → no delete policy |
| `test_rls_enable_on_all_tables` | diff | All tables get ENABLE + FORCE RLS |
| `test_rls_no_access_blocks_all` | integration | Table without rules → all access blocked |
| `test_rls_blocks_unauthorized_read` | integration | User A can't read User B's private rows |
| `test_rls_allows_authorized_read` | integration | Owner can read own rows |
| `test_rls_public_read_anon` | integration | Anonymous can read public rows |
| `test_rls_blocks_unauthorized_write` | integration | User A can't update User B's rows |
| `test_rls_update_cant_transfer_ownership` | integration | Can't SET author_id to another user (WITH CHECK) |
| `test_rls_set_local_authenticated` | integration | JWT → SET LOCAL role authenticated + user_id |
| `test_rls_set_local_anon` | integration | No JWT → SET LOCAL role anon |
| `test_sql_guard_blocks_set_role` | sql_guard | SET ROLE / RESET ROLE blocked |
| `test_realtime_filters_by_permissions` | Elixir | Subscriber only gets notifications they're authorized to see |

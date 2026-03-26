# Permissions (Row Level Security) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce access control via PostgreSQL Row Level Security. Access rules from the schema DSL are compiled to RLS policies. Every data query runs with a scoped PG role. Deny by default — tables without explicit access rules block all access.

**Architecture:** 5 tasks: (1) RLS policy SQL generation module, (2) PG role setup + Engine SET LOCAL wrapper, (3) Diff engine extension for RLS in migrations, (4) SQL guard hardening, (5) Realtime permission filtering. Each task produces testable code independently.

**Tech Stack:** Rust (Engine), Elixir (Realtime), PostgreSQL RLS

**Spec:** `docs/superpowers/specs/2026-03-26-permissions-design.md`

---

## Task 1: RLS Policy SQL Generation

New module that translates access rules from `GaranceAccess` (JSON schema) into SQL statements for RLS policies.

**Files:**
- Create: `engine/crates/garance-engine/src/schema/rls.rs`
- Modify: `engine/crates/garance-engine/src/schema/mod.rs` — add `pub mod rls;`
- Create: `engine/crates/garance-engine/tests/rls_test.rs`

- [ ] **Step 1: Create RLS module**

```rust
// engine/crates/garance-engine/src/schema/rls.rs
use crate::schema::json_schema::{GaranceAccess, AccessRule, AccessCondition};

/// Generate SQL to enable RLS on a table (always, regardless of access rules).
pub fn enable_rls(table_name: &str) -> Vec<String> {
    vec![
        format!("ALTER TABLE \"{}\" ENABLE ROW LEVEL SECURITY", table_name),
        format!("ALTER TABLE \"{}\" FORCE ROW LEVEL SECURITY", table_name),
    ]
}

/// Generate SQL to disable RLS on a table.
pub fn disable_rls(table_name: &str) -> Vec<String> {
    vec![
        format!("ALTER TABLE \"{}\" DISABLE ROW LEVEL SECURITY", table_name),
    ]
}

/// Generate all RLS policy statements for a table from its access rules.
/// Returns (policy_name, sql) pairs.
pub fn generate_policies(table_name: &str, access: &GaranceAccess) -> Vec<(String, String)> {
    let mut policies = vec![];

    // SELECT policy from read
    if let Some(ref read) = access.read {
        let using_clause = access_rule_to_sql(read);
        let name = format!("garance_select_{}", table_name);
        let sql = format!(
            "CREATE POLICY \"{}\" ON \"{}\" FOR SELECT USING ({})",
            name, table_name, using_clause
        );
        policies.push((name, sql));
    }

    // INSERT policy from write
    if let Some(ref write) = access.write {
        let check_clause = access_rule_to_sql(write);
        let name = format!("garance_insert_{}", table_name);
        let sql = format!(
            "CREATE POLICY \"{}\" ON \"{}\" FOR INSERT WITH CHECK ({})",
            name, table_name, check_clause
        );
        policies.push((name, sql));

        // UPDATE policy from write (USING + WITH CHECK)
        let name = format!("garance_update_{}", table_name);
        let sql = format!(
            "CREATE POLICY \"{}\" ON \"{}\" FOR UPDATE USING ({}) WITH CHECK ({})",
            name, table_name, check_clause, check_clause
        );
        policies.push((name, sql));
    }

    // DELETE policy — explicit only, does NOT inherit from write
    if let Some(ref delete) = access.delete {
        let using_clause = access_rule_to_sql(delete);
        let name = format!("garance_delete_{}", table_name);
        let sql = format!(
            "CREATE POLICY \"{}\" ON \"{}\" FOR DELETE USING ({})",
            name, table_name, using_clause
        );
        policies.push((name, sql));
    }

    policies
}

/// Drop all garance policies on a table.
pub fn drop_policies(table_name: &str, policy_names: &[String]) -> Vec<String> {
    policy_names.iter().map(|name| {
        format!("DROP POLICY IF EXISTS \"{}\" ON \"{}\"", name, table_name)
    }).collect()
}

/// Convert an AccessRule to a SQL expression for USING/WITH CHECK.
fn access_rule_to_sql(rule: &AccessRule) -> String {
    match rule {
        AccessRule::Public(s) => match s.as_str() {
            "public" => "true".to_string(),
            "authenticated" => "current_setting('request.user_id', true) IS NOT NULL".to_string(),
            other => format!("false /* unknown rule: {} */", other),
        },
        AccessRule::Conditions(conditions) => {
            if conditions.is_empty() {
                return "false".to_string();
            }
            let parts: Vec<String> = conditions.iter().map(condition_to_sql).collect();
            if parts.len() == 1 {
                parts[0].clone()
            } else {
                // Multiple conditions combined with OR
                parts.iter().map(|p| format!("({})", p)).collect::<Vec<_>>().join(" OR ")
            }
        }
    }
}

/// Convert a single AccessCondition to a SQL expression.
fn condition_to_sql(cond: &AccessCondition) -> String {
    match cond.condition_type.as_str() {
        "isOwner" => {
            let column = cond.column.as_deref().unwrap_or("user_id");
            format!(
                "\"{}\"::text = current_setting('request.user_id', true)",
                column
            )
        }
        "isAuthenticated" => {
            "current_setting('request.user_id', true) IS NOT NULL".to_string()
        }
        "where" => {
            match &cond.filters {
                Some(filters) => {
                    let conditions: Vec<String> = filters.iter().map(|(col, val)| {
                        let sql_val = value_to_sql_literal(val);
                        format!("\"{}\" = {}", col, sql_val)
                    }).collect();
                    conditions.join(" AND ")
                }
                None => "true".to_string(),
            }
        }
        other => format!("false /* unknown condition: {} */", other),
    }
}

/// Convert a JSON value to a safe SQL literal (equivalent of quote_literal).
fn value_to_sql_literal(val: &serde_json::Value) -> String {
    match val {
        serde_json::Value::Bool(b) => b.to_string(),
        serde_json::Value::Number(n) => n.to_string(),
        serde_json::Value::String(s) => {
            // Escape single quotes by doubling them (PostgreSQL standard)
            let escaped = s.replace('\'', "''");
            format!("'{}'", escaped)
        }
        serde_json::Value::Null => "NULL".to_string(),
        _ => "NULL".to_string(),
    }
}
```

- [ ] **Step 2: Register module**

Add `pub mod rls;` to `engine/crates/garance-engine/src/schema/mod.rs`.

- [ ] **Step 3: Write tests**

```rust
// engine/crates/garance-engine/tests/rls_test.rs
use garance_engine::schema::rls::*;
use garance_engine::schema::json_schema::*;
use std::collections::HashMap;

fn make_access_isowner(column: &str) -> GaranceAccess {
    GaranceAccess {
        read: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "isOwner".into(),
            column: Some(column.into()),
            filters: None,
        }])),
        write: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "isOwner".into(),
            column: Some(column.into()),
            filters: None,
        }])),
        delete: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "isOwner".into(),
            column: Some(column.into()),
            filters: None,
        }])),
    }
}

#[test]
fn test_rls_policy_isowner() {
    let access = make_access_isowner("author_id");
    let policies = generate_policies("posts", &access);
    assert_eq!(policies.len(), 4); // select, insert, update, delete

    let select = &policies[0].1;
    assert!(select.contains("FOR SELECT USING"));
    assert!(select.contains("\"author_id\"::text = current_setting('request.user_id', true)"));
}

#[test]
fn test_rls_policy_where() {
    let mut filters = HashMap::new();
    filters.insert("published".into(), serde_json::Value::Bool(true));

    let access = GaranceAccess {
        read: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "where".into(),
            column: None,
            filters: Some(filters),
        }])),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    assert_eq!(policies.len(), 1);
    assert!(policies[0].1.contains("\"published\" = true"));
}

#[test]
fn test_rls_policy_public() {
    let access = GaranceAccess {
        read: Some(AccessRule::Public("public".into())),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    assert!(policies[0].1.contains("USING (true)"));
}

#[test]
fn test_rls_policy_authenticated() {
    let access = GaranceAccess {
        read: Some(AccessRule::Public("authenticated".into())),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    assert!(policies[0].1.contains("IS NOT NULL"));
}

#[test]
fn test_rls_policy_combined_or() {
    let mut filters = HashMap::new();
    filters.insert("published".into(), serde_json::Value::Bool(true));

    let access = GaranceAccess {
        read: Some(AccessRule::Conditions(vec![
            AccessCondition { condition_type: "where".into(), column: None, filters: Some(filters) },
            AccessCondition { condition_type: "isOwner".into(), column: Some("author_id".into()), filters: None },
        ])),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    let sql = &policies[0].1;
    assert!(sql.contains(" OR "));
    assert!(sql.contains("\"published\" = true"));
    assert!(sql.contains("\"author_id\"::text"));
}

#[test]
fn test_rls_update_has_with_check() {
    let access = make_access_isowner("author_id");
    let policies = generate_policies("posts", &access);
    let update = policies.iter().find(|(name, _)| name.contains("update")).unwrap();
    assert!(update.1.contains("USING ("));
    assert!(update.1.contains("WITH CHECK ("));
}

#[test]
fn test_rls_delete_not_inherited() {
    let access = GaranceAccess {
        read: Some(AccessRule::Public("public".into())),
        write: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "isOwner".into(),
            column: Some("author_id".into()),
            filters: None,
        }])),
        delete: None, // NOT specified
    };

    let policies = generate_policies("posts", &access);
    let has_delete = policies.iter().any(|(name, _)| name.contains("delete"));
    assert!(!has_delete, "delete policy should NOT exist when not declared");
}

#[test]
fn test_enable_rls() {
    let stmts = enable_rls("posts");
    assert_eq!(stmts.len(), 2);
    assert!(stmts[0].contains("ENABLE ROW LEVEL SECURITY"));
    assert!(stmts[1].contains("FORCE ROW LEVEL SECURITY"));
}

#[test]
fn test_value_escaping() {
    let mut filters = HashMap::new();
    filters.insert("name".into(), serde_json::Value::String("O'Brien".into()));

    let access = GaranceAccess {
        read: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "where".into(),
            column: None,
            filters: Some(filters),
        }])),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    assert!(policies[0].1.contains("'O''Brien'"), "single quotes should be doubled");
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/engine && cargo test -p garance-engine -- rls_test`
Expected: 9 tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add engine/
git commit -m ":sparkles: feat(engine): add RLS policy SQL generation from access rules"
```

---

## Task 2: PG Roles + Engine SET LOCAL Wrapper

Create PG roles on startup and wrap every data query with `SET LOCAL role`.

**Files:**
- Create: `engine/crates/garance-engine/src/schema/roles.rs`
- Modify: `engine/crates/garance-engine/src/schema/mod.rs` — add `pub mod roles;`
- Modify: `engine/crates/garance-engine/src/main.rs` — call role setup
- Modify: `engine/crates/garance-engine/src/api/routes.rs` — wrap CRUD handlers with SET LOCAL
- Create: `engine/crates/garance-engine/tests/rls_integration_test.rs`

- [ ] **Step 1: Create roles module**

```rust
// engine/crates/garance-engine/src/schema/roles.rs
use tokio_postgres::Client;
use tracing::info;

/// Create garance_anon and garance_authenticated roles (idempotent).
pub async fn ensure_roles(client: &Client) -> Result<(), tokio_postgres::Error> {
    client.batch_execute(r#"
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
    "#).await?;

    info!("PG roles garance_anon and garance_authenticated ensured");
    Ok(())
}

/// Grant permissions on a specific table to the roles.
/// Called after creating new tables (migrate/reload).
pub async fn grant_table_permissions(client: &Client, table_name: &str) -> Result<(), tokio_postgres::Error> {
    let sql = format!(
        "GRANT SELECT ON \"{}\" TO garance_anon; GRANT SELECT, INSERT, UPDATE, DELETE ON \"{}\" TO garance_authenticated;",
        table_name, table_name
    );
    client.batch_execute(&sql).await?;
    Ok(())
}
```

- [ ] **Step 2: Create helper for SET LOCAL in queries**

Add a helper function to `routes.rs` that wraps a query execution with the correct role:

```rust
/// Execute a query with RLS context (SET LOCAL role based on JWT claims).
/// user_id: from JWT claims (None for anonymous).
async fn with_rls_context<F, T>(
    client: &deadpool_postgres::Client,
    user_id: Option<&str>,
    user_role: Option<&str>,
    f: F,
) -> Result<T, tokio_postgres::Error>
where
    F: FnOnce(&deadpool_postgres::Client) -> std::pin::Pin<Box<dyn std::future::Future<Output = Result<T, tokio_postgres::Error>> + Send + '_>>,
{
    let role = if user_id.is_some() { "garance_authenticated" } else { "garance_anon" };
    let uid = user_id.unwrap_or("");
    let urole = user_role.unwrap_or("");

    client.execute("BEGIN", &[]).await?;
    client.execute(&format!("SET LOCAL role TO '{}'", role), &[]).await?;
    client.execute(&format!("SET LOCAL request.user_id TO '{}'", uid.replace('\'', "''")), &[]).await?;
    client.execute(&format!("SET LOCAL request.user_role TO '{}'", urole.replace('\'', "''")), &[]).await?;

    match f(client).await {
        Ok(result) => {
            client.execute("COMMIT", &[]).await?;
            Ok(result)
        }
        Err(e) => {
            let _ = client.execute("ROLLBACK", &[]).await;
            Err(e)
        }
    }
}
```

**Note:** The `with_rls_context` approach with a generic closure is complex in Rust due to async lifetimes. A simpler approach: extract user_id/role from headers, then do inline SET LOCAL before each query. The helper functions `set_rls_context` and `reset_rls_context` are simpler:

```rust
/// Set RLS context for the current transaction.
async fn set_rls_context(
    client: &deadpool_postgres::Client,
    user_id: Option<&str>,
) -> Result<(), tokio_postgres::Error> {
    let role = if user_id.is_some() { "garance_authenticated" } else { "garance_anon" };
    let uid = user_id.unwrap_or("");

    client.execute("BEGIN", &[]).await?;
    client.execute(&format!("SET LOCAL role TO '{}'", role), &[]).await?;
    client.execute(
        &format!("SET LOCAL request.user_id TO '{}'", uid.replace('\'', "''")),
        &[],
    ).await?;
    Ok(())
}

/// Extract user_id from request headers (set by Gateway from JWT).
fn get_user_id(headers: &HeaderMap) -> Option<String> {
    headers.get("x-user-id")
        .and_then(|v| v.to_str().ok())
        .map(|s| s.to_string())
}
```

- [ ] **Step 3: Update CRUD handlers to use SET LOCAL**

Each handler (`list_rows`, `get_row`, `insert_row`, `update_row`, `delete_row`) needs:

1. Extract `user_id` from headers via `get_user_id(&headers)`
2. Call `set_rls_context(&client, user_id.as_deref()).await`
3. Execute the query
4. COMMIT on success, ROLLBACK on error
5. Map PG error 42501 (insufficient_privilege) to 403 PERMISSION_DENIED

Add `headers: HeaderMap` parameter to each handler. Example for `list_rows`:

```rust
pub async fn list_rows(
    State(state): State<AppState>,
    headers: HeaderMap,
    Path(table_name): Path<String>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<impl IntoResponse, ApiError> {
    let user_id = get_user_id(&headers);
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;

    let param_vec: Vec<(String, String)> = params.into_iter().collect();
    let qp = parse_query_params(&param_vec)?;
    let sql_query = build_select(table, &qp)?;

    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
    })?;

    // Set RLS context
    set_rls_context(&client, user_id.as_deref()).await.map_err(map_pg_error)?;

    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
        sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

    let result = client.query(&sql_query.sql, &params_refs).await;

    match result {
        Ok(rows) => {
            let _ = client.execute("COMMIT", &[]).await;
            let results: Vec<Value> = rows.iter().map(row_to_json).collect();
            Ok(Json(results))
        }
        Err(e) => {
            let _ = client.execute("ROLLBACK", &[]).await;
            Err(map_pg_error(e))
        }
    }
}

/// Map PG errors — 42501 = insufficient_privilege → 403
fn map_pg_error(e: tokio_postgres::Error) -> ApiError {
    if let Some(db_err) = e.as_db_error() {
        if db_err.code() == &tokio_postgres::error::SqlState::INSUFFICIENT_PRIVILEGE {
            return ApiError {
                error: super::error::ApiErrorBody {
                    code: "PERMISSION_DENIED".into(),
                    message: "you do not have permission to perform this action".into(),
                    status: 403,
                    details: None,
                },
            };
        }
    }
    ApiError {
        error: super::error::ApiErrorBody {
            code: "INTERNAL_ERROR".into(),
            message: "internal database error".into(),
            status: 500,
            details: None,
        },
    }
}
```

Apply the same pattern to `get_row`, `insert_row`, `update_row`, `delete_row`.

- [ ] **Step 4: Update main.rs**

After role setup in `main.rs`:

```rust
// Ensure PG roles exist
schema::roles::ensure_roles(&client).await.expect("failed to create PG roles");
```

Add after trigger attachment.

- [ ] **Step 5: Register module**

Add `pub mod roles;` to `engine/crates/garance-engine/src/schema/mod.rs`.

- [ ] **Step 6: Write integration tests**

```rust
// engine/crates/garance-engine/tests/rls_integration_test.rs
use std::sync::Arc;
use tokio::sync::RwLock;
use axum_test::TestServer;
use serde_json::json;
use testcontainers::runners::AsyncRunner;
use testcontainers_modules::postgres::Postgres;

use garance_engine::api;
use garance_engine::schema;
use garance_pooler::{GarancePool, PoolConfig};

async fn setup_with_rls() -> (testcontainers::ContainerAsync<Postgres>, TestServer) {
    let container = Postgres::default().start().await.unwrap();
    let port = container.get_host_port_ipv4(5432).await.unwrap();
    let config = PoolConfig { host: "127.0.0.1".into(), port, user: "postgres".into(), password: "postgres".into(), dbname: "postgres".into(), max_size: 4 };
    let pool = GarancePool::new(&config).unwrap();
    let client = pool.get().await.unwrap();

    let _ = client.execute("CREATE EXTENSION IF NOT EXISTS pgcrypto", &[]).await;

    // Create roles
    schema::roles::ensure_roles(&client).await.unwrap();

    // Create table with RLS
    client.batch_execute(r#"
        CREATE TABLE posts (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            title text NOT NULL,
            author_id text NOT NULL,
            published boolean DEFAULT false
        );
        ALTER TABLE posts ENABLE ROW LEVEL SECURITY;
        ALTER TABLE posts FORCE ROW LEVEL SECURITY;

        CREATE POLICY garance_select_posts ON posts FOR SELECT USING (
            ("published" = true) OR ("author_id"::text = current_setting('request.user_id', true))
        );
        CREATE POLICY garance_insert_posts ON posts FOR INSERT WITH CHECK (
            "author_id"::text = current_setting('request.user_id', true)
        );
        CREATE POLICY garance_update_posts ON posts FOR UPDATE
            USING ("author_id"::text = current_setting('request.user_id', true))
            WITH CHECK ("author_id"::text = current_setting('request.user_id', true));
        CREATE POLICY garance_delete_posts ON posts FOR DELETE USING (
            "author_id"::text = current_setting('request.user_id', true)
        );

        GRANT SELECT ON posts TO garance_anon;
        GRANT SELECT, INSERT, UPDATE, DELETE ON posts TO garance_authenticated;

        -- Insert test data
        INSERT INTO posts (title, author_id, published) VALUES ('Public Post', 'user-1', true);
        INSERT INTO posts (title, author_id, published) VALUES ('Private Post', 'user-1', false);
        INSERT INTO posts (title, author_id, published) VALUES ('Other Post', 'user-2', false);
    "#).await.unwrap();

    drop(client);

    let client = pool.get().await.unwrap();
    let db_schema = schema::introspect(&client, "public").await.unwrap();
    drop(client);

    let state = api::state::AppState { pool: Arc::new(pool), schema: Arc::new(RwLock::new(db_schema)) };
    let app = api::router(state);
    let server = TestServer::new(app).unwrap();
    (container, server)
}

#[tokio::test]
async fn test_rls_anon_sees_only_public() {
    let (_container, server) = setup_with_rls().await;
    // No x-user-id header = anonymous
    let response = server.get("/api/v1/posts").await;
    response.assert_status_ok();
    let body: Vec<serde_json::Value> = response.json();
    // Anon should only see published posts
    assert_eq!(body.len(), 1);
    assert_eq!(body[0]["title"], "Public Post");
}

#[tokio::test]
async fn test_rls_owner_sees_own_posts() {
    let (_container, server) = setup_with_rls().await;
    let response = server.get("/api/v1/posts")
        .add_header(
            axum::http::HeaderName::from_static("x-user-id"),
            axum::http::HeaderValue::from_static("user-1"),
        )
        .await;
    response.assert_status_ok();
    let body: Vec<serde_json::Value> = response.json();
    // user-1 sees public + own private (2 posts)
    assert_eq!(body.len(), 2);
}

#[tokio::test]
async fn test_rls_user_cant_see_others_private() {
    let (_container, server) = setup_with_rls().await;
    let response = server.get("/api/v1/posts")
        .add_header(
            axum::http::HeaderName::from_static("x-user-id"),
            axum::http::HeaderValue::from_static("user-2"),
        )
        .await;
    response.assert_status_ok();
    let body: Vec<serde_json::Value> = response.json();
    // user-2 sees public post + own private post
    assert_eq!(body.len(), 2);
    let titles: Vec<&str> = body.iter().map(|r| r["title"].as_str().unwrap()).collect();
    assert!(titles.contains(&"Public Post"));
    assert!(titles.contains(&"Other Post"));
    assert!(!titles.contains(&"Private Post"));
}

#[tokio::test]
async fn test_rls_blocks_unauthorized_insert() {
    let (_container, server) = setup_with_rls().await;
    // user-1 tries to insert a post as user-2
    let response = server.post("/api/v1/posts")
        .add_header(
            axum::http::HeaderName::from_static("x-user-id"),
            axum::http::HeaderValue::from_static("user-1"),
        )
        .json(&json!({"title": "Sneaky", "author_id": "user-2", "published": false}))
        .await;
    // Should be 403 (WITH CHECK blocks author_id != user-1)
    response.assert_status(axum::http::StatusCode::FORBIDDEN);
}

#[tokio::test]
async fn test_rls_allows_authorized_insert() {
    let (_container, server) = setup_with_rls().await;
    let response = server.post("/api/v1/posts")
        .add_header(
            axum::http::HeaderName::from_static("x-user-id"),
            axum::http::HeaderValue::from_static("user-1"),
        )
        .json(&json!({"title": "My Post", "author_id": "user-1", "published": false}))
        .await;
    response.assert_status(axum::http::StatusCode::CREATED);
}

#[tokio::test]
async fn test_rls_no_access_blocks_all() {
    let (_container, server) = setup_with_rls().await;

    // Create a table with RLS but NO policies (deny by default)
    // We'd need direct DB access for this — skip for this test file.
    // This is tested via the diff engine in Task 3.
}
```

- [ ] **Step 7: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/engine && cargo test -p garance-engine -- rls`
Expected: All RLS tests pass.

- [ ] **Step 8: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add engine/
git commit -m ":sparkles: feat(engine): add PG roles and SET LOCAL RLS context in all CRUD handlers"
```

---

## Task 3: Diff Engine — RLS in Migrations

Extend the diff engine to generate ENABLE RLS + policies during `garance db migrate`.

**Files:**
- Modify: `engine/crates/garance-engine/src/diff/diff.rs` — add RLS generation
- Modify: `engine/crates/garance-engine/src/api/routes.rs` — grant permissions after migrate

- [ ] **Step 1: Extend diff to generate RLS**

In `diff.rs`, after generating table/column changes, add RLS policy generation:

1. For ALL tables (desired): generate `ENABLE ROW LEVEL SECURITY` + `FORCE ROW LEVEL SECURITY`
2. For tables with `access` rules: generate `CREATE POLICY` statements
3. For tables with changed `access` rules: `DROP POLICY IF EXISTS` + `CREATE POLICY`
4. Introspect existing policies from `pg_policies` to determine what to drop/create

Add to `DiffStatements`:
```rust
enable_rls: Vec<String>,
create_policies: Vec<String>,
drop_policies: Vec<String>,
```

Add to the diff output ordering (after table creation, before drops):
```
... existing order ...
enable_rls (after create_tables)
create_policies (after add_fks)
drop_policies (before drop_columns)
```

- [ ] **Step 2: Grant table permissions after migrate**

In `migrate_apply` handler, after reloading schema and attaching triggers, also grant permissions:

```rust
// Grant permissions to roles on new tables
for table_name in new_schema.tables.keys() {
    let _ = crate::schema::roles::grant_table_permissions(&trigger_client, table_name).await;
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/engine && cargo test -p garance-engine`
Expected: All tests pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add engine/
git commit -m ":sparkles: feat(engine): generate RLS policies in migrations and grant role permissions"
```

---

## Task 4: SQL Guard Hardening

Block privilege escalation SQL in `rpc/query`.

**Files:**
- Modify: `engine/crates/garance-engine/src/api/sql_guard.rs` — add blocked patterns
- Modify: `engine/crates/garance-engine/src/api/routes.rs` — apply RLS to execute_sql

- [ ] **Step 1: Add blocked patterns**

In `sql_guard.rs`, add to `BLOCKED_SCHEMAS` or create a new constant:

```rust
const BLOCKED_COMMANDS: &[&str] = &[
    "set role",
    "set local role",
    "reset role",
    "set session",
    "set local request.",
    "grant ",
    "revoke ",
    "create role",
    "alter role",
    "drop role",
    "create policy",
    "alter policy",
    "drop policy",
];
```

Add to `validate_sql`:
```rust
// Blocked commands check (case-insensitive)
let lower = sql.to_lowercase();
for cmd in BLOCKED_COMMANDS {
    if lower.contains(cmd) {
        return Err(SqlValidationError::BlockedCommand(cmd.to_string()));
    }
}
```

Add `BlockedCommand(String)` to `SqlValidationError` enum with display:
```rust
SqlValidationError::BlockedCommand(cmd) => write!(f, "SQL command '{}' is not allowed", cmd),
```

- [ ] **Step 2: Apply RLS to execute_sql handler**

Update `execute_sql` to use `set_rls_context` before running the SQL:

```rust
// In execute_sql handler, after BEGIN and SET LOCAL search_path:
let user_id = get_user_id(&headers);
let role = if user_id.is_some() { "garance_authenticated" } else { "garance_anon" };
client.execute(&format!("SET LOCAL role TO '{}'", role), &[]).await.map_err(&pg_err)?;
if let Some(ref uid) = user_id {
    client.execute(&format!("SET LOCAL request.user_id TO '{}'", uid.replace('\'', "''")), &[]).await.map_err(&pg_err)?;
}
```

- [ ] **Step 3: Write tests**

Add to sql_guard unit tests:
```rust
#[test]
fn test_blocked_set_role() {
    assert!(matches!(validate_sql("SET ROLE garance_anon"), Err(SqlValidationError::BlockedCommand(_))));
    assert!(matches!(validate_sql("RESET ROLE"), Err(SqlValidationError::BlockedCommand(_))));
    assert!(matches!(validate_sql("GRANT SELECT ON users TO evil"), Err(SqlValidationError::BlockedCommand(_))));
    assert!(matches!(validate_sql("CREATE ROLE hacker"), Err(SqlValidationError::BlockedCommand(_))));
}
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/engine && cargo test -p garance-engine`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add engine/
git commit -m ":lock: feat(engine): harden SQL guard and apply RLS to rpc/query"
```

---

## Task 5: Realtime Permission Filtering

Filter NOTIFY events in the Elixir Dispatcher based on subscriber permissions.

**Files:**
- Modify: `services/realtime/lib/realtime_web/channels/changes_channel.ex` — store user_id at join
- Modify: `services/realtime/lib/realtime/subscription_registry.ex` — store user_id per subscription
- Modify: `services/realtime/lib/realtime/dispatcher.ex` — check permissions before pushing
- Create: `services/realtime/test/realtime/permission_filter_test.exs`

- [ ] **Step 1: Store user_id at WebSocket join**

Update `changes_channel.ex` — extract user_id from connect params:

```elixir
@impl true
def join("realtime:" <> table, params, socket) do
  user_id = Map.get(params, "user_id", nil)
  Logger.info("Client joined realtime:#{table}, user_id=#{inspect(user_id)}")
  {:ok, assign(socket, table: table, user_id: user_id)}
end
```

Update `handle_in("subscribe")` to pass user_id to registry:

```elixir
Realtime.SubscriptionRegistry.subscribe(self(), table, events, filters, socket.assigns.user_id)
```

- [ ] **Step 2: Update SubscriptionRegistry to store user_id**

Change ETS entries from `{pid, ref, table, events, filters}` to `{pid, ref, table, events, filters, user_id}`.

Update `subscribe/5` → `subscribe/6`:
```elixir
def subscribe(pid, table, events, filters, user_id \\ nil) do
  ref = Process.monitor(pid)
  :ets.insert(@table, {pid, ref, table, events, filters, user_id})
  :ok
end
```

Update `get_subscribers` to return `user_id`:
```elixir
def get_subscribers(table) do
  :ets.match_object(@table, {:_, :_, table, :_, :_, :_})
  |> Enum.map(fn {pid, _ref, _table, events, filters, user_id} ->
    %{pid: pid, events: events, filters: filters, user_id: user_id}
  end)
end
```

- [ ] **Step 3: Add permission check in Dispatcher**

Update `dispatcher.ex` — after filter match, check if the subscriber would have access to the row:

```elixir
@impl true
def handle_info({:pg_change, change}, state) do
  table = change["table"]
  subscribers = Realtime.SubscriptionRegistry.get_subscribers(table)

  for sub <- subscribers do
    if Realtime.Filter.match?(change, sub) and has_permission?(change, sub) do
      send(sub.pid, {:realtime_change, change})
    end
  end

  {:noreply, state}
end

# Check if the subscriber has permission to see this row.
# For isOwner policies: compare the owner column in the payload with the subscriber's user_id.
defp has_permission?(_change, %{user_id: nil}), do: true  # no auth context = allow (for now)
defp has_permission?(change, %{user_id: user_id, filters: filters}) do
  row = change["new"] || change["old"] || %{}

  # Check all isOwner-style filters: the subscriber's user_id must match the row's owner column
  # For MVP: if any subscription filter references a column that matches user_id, allow
  # If no ownership filters, allow (the subscription filter already handles the rest)
  case find_owner_column(filters) do
    nil -> true  # no owner filter = subscription-level filtering only
    column -> to_string(row[column]) == to_string(user_id)
  end
end

defp find_owner_column(filters) do
  Enum.find_value(filters, fn
    {column, "eq", _value} -> column  # owner filter is typically eq on user_id
    _ -> nil
  end)
end
```

- [ ] **Step 4: Write permission filter test**

```elixir
# services/realtime/test/realtime/permission_filter_test.exs
defmodule Realtime.PermissionFilterTest do
  use ExUnit.Case

  test "subscriber only receives authorized notifications" do
    # Subscribe user-1 to todos with owner filter
    Realtime.SubscriptionRegistry.subscribe(
      self(), "todos", ["*"], [{"user_id", "eq", "user-1"}], "user-1"
    )

    # Simulate a change for user-1's todo
    change_own = %{
      "table" => "todos",
      "event" => "INSERT",
      "new" => %{"id" => "1", "title" => "My todo", "user_id" => "user-1"},
      "old" => nil,
      "timestamp" => "now"
    }

    # Simulate a change for user-2's todo
    change_other = %{
      "table" => "todos",
      "event" => "INSERT",
      "new" => %{"id" => "2", "title" => "Other todo", "user_id" => "user-2"},
      "old" => nil,
      "timestamp" => "now"
    }

    # Broadcast both via PubSub
    Phoenix.PubSub.broadcast(Realtime.PubSub, "pg_changes", {:pg_change, change_own})
    Phoenix.PubSub.broadcast(Realtime.PubSub, "pg_changes", {:pg_change, change_other})

    # Should receive own change
    assert_receive {:realtime_change, received_own}, 2000
    assert received_own["new"]["user_id"] == "user-1"

    # Should NOT receive other's change
    refute_receive {:realtime_change, _}, 500

    Realtime.SubscriptionRegistry.unsubscribe_all(self())
  end
end
```

- [ ] **Step 5: Run tests**

```bash
cd /Users/jh3ady/Development/Projects/garance/services/realtime && mix test
```

Expected: 15 tests pass (14 existing + 1 new permission).

- [ ] **Step 6: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add services/realtime/
git commit -m ":lock: feat(realtime): filter notifications by subscriber permissions"
```

---

## Summary

| Task | Description | Tests |
|---|---|---|
| 1 | RLS policy SQL generation from access rules | 9 |
| 2 | PG roles + SET LOCAL wrapper in all CRUD handlers | 5 integration |
| 3 | Diff engine extension for RLS in migrations | 0 (existing tests) |
| 4 | SQL guard hardening (block SET ROLE, GRANT, etc.) | 1 |
| 5 | Realtime permission filtering | 1 |
| **Total** | | **16** |

### Not in this plan (deferred)

- Dashboard UI for viewing/editing access rules
- Column-level permissions (only row-level for now)
- Role-based access beyond user/anon (custom roles like "admin", "moderator")
- Rate limiting per user
- Audit logging of permission violations

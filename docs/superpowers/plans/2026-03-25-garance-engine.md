# Garance Engine — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Garance Query Engine in Rust — the foundational service that introspects PostgreSQL schemas, generates REST-compatible API responses, manages connection pooling, reads declarative schema JSON, and generates client types.

**Architecture:** Cargo workspace with 3 crates: `garance-pooler` (connection pool with `search_path` support), `garance-engine` (introspection, query building, HTTP+gRPC server), `garance-codegen` (TypeScript type generation). The Engine exposes both an HTTP API (for direct dev access) and a gRPC interface (for Gateway communication in production). All PG queries go through the integrated pooler.

**Tech Stack:** Rust 1.85+ (stable), axum 0.8 (HTTP), tokio-postgres + deadpool-postgres (PG), serde (JSON), testcontainers (integration tests). gRPC (tonic) is deferred to Plan 4 (Gateway).

**Spec:** `docs/superpowers/specs/2026-03-25-garance-baas-design.md` (sections 3, 5, 13, 14, 15)

---

## Task 1: Cargo Workspace Setup

**Files:**
- Create: `engine/Cargo.toml` (workspace root)
- Create: `engine/crates/garance-pooler/Cargo.toml`
- Create: `engine/crates/garance-pooler/src/lib.rs`
- Create: `engine/crates/garance-engine/Cargo.toml`
- Create: `engine/crates/garance-engine/src/lib.rs`
- Create: `engine/crates/garance-engine/src/main.rs`
- Create: `engine/crates/garance-codegen/Cargo.toml`
- Create: `engine/crates/garance-codegen/src/lib.rs`
- Create: `engine/.gitignore`
- Create: `engine/rust-toolchain.toml`

- [ ] **Step 1: Create workspace Cargo.toml**

```toml
# engine/Cargo.toml
[workspace]
resolver = "2"
members = [
    "crates/garance-pooler",
    "crates/garance-engine",
    "crates/garance-codegen",
]

[workspace.package]
version = "0.1.0"
edition = "2021"
license = "Apache-2.0"
repository = "https://github.com/garancehq/garance"

[workspace.dependencies]
tokio = { version = "1", features = ["full"] }
tokio-postgres = { version = "0.7", features = ["with-serde_json-1", "with-uuid-1", "with-chrono-0_4"] }
deadpool-postgres = "0.14"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
uuid = { version = "1", features = ["v4", "serde"] }
chrono = { version = "0.4", features = ["serde"] }
url = "2"
thiserror = "2"
tracing = "0.1"
tracing-subscriber = { version = "0.3", features = ["json", "env-filter"] }
axum = "0.8"
```

- [ ] **Step 2: Create garance-pooler Cargo.toml**

```toml
# engine/crates/garance-pooler/Cargo.toml
[package]
name = "garance-pooler"
version.workspace = true
edition.workspace = true

[dependencies]
tokio-postgres.workspace = true
deadpool-postgres.workspace = true
tokio.workspace = true
serde.workspace = true
url.workspace = true
thiserror.workspace = true
tracing.workspace = true

[dev-dependencies]
testcontainers = "0.23"
testcontainers-modules = { version = "0.11", features = ["postgres"] }
```

- [ ] **Step 3: Create garance-engine Cargo.toml**

```toml
# engine/crates/garance-engine/Cargo.toml
[package]
name = "garance-engine"
version.workspace = true
edition.workspace = true

[dependencies]
garance-pooler = { path = "../garance-pooler" }
garance-codegen = { path = "../garance-codegen" }
tokio-postgres.workspace = true
tokio.workspace = true
serde.workspace = true
serde_json.workspace = true
uuid.workspace = true
chrono.workspace = true
thiserror.workspace = true
tracing.workspace = true
tracing-subscriber.workspace = true
axum.workspace = true

[dev-dependencies]
testcontainers = "0.23"
testcontainers-modules = { version = "0.11", features = ["postgres"] }
axum-test = "17"
```

- [ ] **Step 4: Create garance-codegen Cargo.toml**

```toml
# engine/crates/garance-codegen/Cargo.toml
[package]
name = "garance-codegen"
version.workspace = true
edition.workspace = true

[dependencies]
serde.workspace = true
serde_json.workspace = true
thiserror.workspace = true
```

- [ ] **Step 5: Create stub source files**

```rust
// engine/crates/garance-pooler/src/lib.rs
//! Garance connection pooler with search_path support.

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}
```

```rust
// engine/crates/garance-engine/src/lib.rs
//! Garance Query Engine — introspection, query building, API serving.

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}
```

```rust
// engine/crates/garance-engine/src/main.rs
fn main() {
    println!("garance-engine v{}", garance_engine::version());
}
```

```rust
// engine/crates/garance-codegen/src/lib.rs
//! Garance codegen — multi-language type generation from PG schemas.

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}
```

- [ ] **Step 6: Create toolchain and gitignore**

```toml
# engine/rust-toolchain.toml
[toolchain]
channel = "stable"
```

```gitignore
# engine/.gitignore
/target
```

Note: `Cargo.lock` MUST be committed (Rust convention for binaries). It will be generated on first `cargo build`.

- [ ] **Step 7: Verify workspace compiles**

Run: `cd engine && cargo build`
Expected: Compiles without errors.

Run: `cargo test`
Expected: 0 tests, all pass.

- [ ] **Step 8: Commit (include Cargo.lock)**

```bash
git add engine/
git commit -m ":tada: feat(engine): initialize Cargo workspace with 3 crates"
```

---

## Task 2: Connection Pooler — Core Pool

**Files:**
- Create: `engine/crates/garance-pooler/src/config.rs`
- Create: `engine/crates/garance-pooler/src/pool.rs`
- Create: `engine/crates/garance-pooler/src/error.rs`
- Modify: `engine/crates/garance-pooler/src/lib.rs`
- Create: `engine/crates/garance-pooler/tests/pool_test.rs`

- [ ] **Step 1: Write the error types**

```rust
// engine/crates/garance-pooler/src/error.rs
use thiserror::Error;

#[derive(Error, Debug)]
pub enum PoolError {
    #[error("failed to connect to PostgreSQL: {0}")]
    Connection(#[from] tokio_postgres::Error),

    #[error("pool exhausted: {0}")]
    Pool(#[from] deadpool_postgres::PoolError),

    #[error("invalid configuration: {0}")]
    Config(String),
}

pub type Result<T> = std::result::Result<T, PoolError>;
```

- [ ] **Step 2: Write the config**

```rust
// engine/crates/garance-pooler/src/config.rs
use serde::Deserialize;

#[derive(Debug, Clone, Deserialize)]
pub struct PoolConfig {
    pub host: String,
    pub port: u16,
    pub user: String,
    pub password: String,
    pub dbname: String,
    #[serde(default = "default_max_size")]
    pub max_size: usize,
}

fn default_max_size() -> usize {
    16
}

impl PoolConfig {
    pub fn from_url(url: &str) -> crate::error::Result<Self> {
        // Parse: postgresql://user:password@host:port/dbname
        let url = url::Url::parse(url).map_err(|e| crate::error::PoolError::Config(e.to_string()))?;
        Ok(Self {
            host: url.host_str().unwrap_or("localhost").to_string(),
            port: url.port().unwrap_or(5432),
            user: url.username().to_string(),
            password: url.password().unwrap_or("").to_string(),
            dbname: url.path().trim_start_matches('/').to_string(),
            max_size: default_max_size(),
        })
    }
}
```

- [ ] **Step 3: Write the pool with search_path support**

```rust
// engine/crates/garance-pooler/src/pool.rs
use deadpool_postgres::{Config, Pool, Runtime, CreatePoolError};
use tokio_postgres::NoTls;
use tracing::info;

use crate::config::PoolConfig;
use crate::error::{PoolError, Result};

pub struct GarancePool {
    inner: Pool,
}

impl GarancePool {
    pub fn new(config: &PoolConfig) -> Result<Self> {
        let mut cfg = Config::new();
        cfg.host = Some(config.host.clone());
        cfg.port = Some(config.port);
        cfg.user = Some(config.user.clone());
        cfg.password = Some(config.password.clone());
        cfg.dbname = Some(config.dbname.clone());
        cfg.pool = Some(deadpool_postgres::PoolConfig {
            max_size: config.max_size,
            ..Default::default()
        });

        let pool = cfg.create_pool(Some(Runtime::Tokio1), NoTls)
            .map_err(|e| PoolError::Config(e.to_string()))?;

        info!(host = %config.host, port = config.port, db = %config.dbname, max_size = config.max_size, "connection pool created");

        Ok(Self { inner: pool })
    }

    /// Get a connection with the default search_path (public).
    pub async fn get(&self) -> Result<deadpool_postgres::Client> {
        let client = self.inner.get().await?;
        Ok(client)
    }

    /// Get a connection with search_path set to a project schema.
    /// Used for multi-tenant isolation in SaaS mode.
    pub async fn get_for_project(&self, project_id: &str) -> Result<deadpool_postgres::Client> {
        let client = self.inner.get().await?;
        let schema = format!("project_{}", project_id);
        client.execute(
            &format!("SET search_path TO \"{}\", public", schema),
            &[],
        ).await?;
        Ok(client)
    }

    pub fn status(&self) -> PoolStatus {
        let status = self.inner.status();
        PoolStatus {
            size: status.size,
            available: status.available,
            waiting: status.waiting,
        }
    }
}

#[derive(Debug)]
pub struct PoolStatus {
    pub size: usize,
    pub available: usize,
    pub waiting: usize,
}
```

- [ ] **Step 4: Update lib.rs to export modules**

```rust
// engine/crates/garance-pooler/src/lib.rs
pub mod config;
pub mod error;
pub mod pool;

pub use config::PoolConfig;
pub use error::PoolError;
pub use pool::GarancePool;
```

- [ ] **Step 5: Write integration test**

```rust
// engine/crates/garance-pooler/tests/pool_test.rs
use garance_pooler::{GarancePool, PoolConfig};
use testcontainers::runners::AsyncRunner;
use testcontainers_modules::postgres::Postgres;

#[tokio::test]
async fn test_pool_connects_and_queries() {
    let container = Postgres::default().start().await.unwrap();
    let port = container.get_host_port_ipv4(5432).await.unwrap();

    let config = PoolConfig {
        host: "127.0.0.1".into(),
        port,
        user: "postgres".into(),
        password: "postgres".into(),
        dbname: "postgres".into(),
        max_size: 4,
    };

    let pool = GarancePool::new(&config).unwrap();
    let client = pool.get().await.unwrap();
    let row = client.query_one("SELECT 1 AS val", &[]).await.unwrap();
    let val: i32 = row.get("val");
    assert_eq!(val, 1);
}

#[tokio::test]
async fn test_pool_search_path_for_project() {
    let container = Postgres::default().start().await.unwrap();
    let port = container.get_host_port_ipv4(5432).await.unwrap();

    let config = PoolConfig {
        host: "127.0.0.1".into(),
        port,
        user: "postgres".into(),
        password: "postgres".into(),
        dbname: "postgres".into(),
        max_size: 4,
    };

    let pool = GarancePool::new(&config).unwrap();
    let client = pool.get().await.unwrap();

    // Create a project schema
    client.execute("CREATE SCHEMA project_abc123", &[]).await.unwrap();
    client.execute("CREATE TABLE project_abc123.users (id serial PRIMARY KEY, name text)", &[]).await.unwrap();

    // Get a connection scoped to the project
    let project_client = pool.get_for_project("abc123").await.unwrap();
    project_client.execute("INSERT INTO users (name) VALUES ('Alice')", &[]).await.unwrap();

    let row = project_client.query_one("SELECT name FROM users WHERE id = 1", &[]).await.unwrap();
    let name: &str = row.get("name");
    assert_eq!(name, "Alice");
}

#[tokio::test]
async fn test_pool_config_from_url() {
    let config = PoolConfig::from_url("postgresql://user:pass@localhost:5432/mydb").unwrap();
    assert_eq!(config.host, "localhost");
    assert_eq!(config.port, 5432);
    assert_eq!(config.user, "user");
    assert_eq!(config.password, "pass");
    assert_eq!(config.dbname, "mydb");
}

#[tokio::test]
async fn test_pool_status() {
    let container = Postgres::default().start().await.unwrap();
    let port = container.get_host_port_ipv4(5432).await.unwrap();

    let config = PoolConfig {
        host: "127.0.0.1".into(),
        port,
        user: "postgres".into(),
        password: "postgres".into(),
        dbname: "postgres".into(),
        max_size: 4,
    };

    let pool = GarancePool::new(&config).unwrap();
    let status = pool.status();
    assert_eq!(status.size, 0); // No connections yet
    assert_eq!(status.waiting, 0);
}
```

- [ ] **Step 6: Run tests**

Run: `cd engine && cargo test -p garance-pooler`
Expected: 4 tests pass. (Requires Docker running for testcontainers.)

- [ ] **Step 7: Commit**

```bash
git add engine/
git commit -m ":sparkles: feat(engine): add connection pooler with search_path support"
```

---

## Task 3: Core Types — Internal Schema Representation

**Files:**
- Create: `engine/crates/garance-engine/src/schema/mod.rs`
- Create: `engine/crates/garance-engine/src/schema/types.rs`
- Modify: `engine/crates/garance-engine/src/lib.rs`
- Create: `engine/crates/garance-engine/tests/schema_types_test.rs`

- [ ] **Step 1: Write the internal schema types**

These types represent the introspected PG schema in memory.

```rust
// engine/crates/garance-engine/src/schema/types.rs
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Schema {
    pub tables: HashMap<String, Table>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Table {
    pub name: String,
    pub schema: String,
    pub columns: Vec<Column>,
    pub primary_key: Option<Vec<String>>,
    pub foreign_keys: Vec<ForeignKey>,
    pub indexes: Vec<Index>,
}

impl Table {
    pub fn column(&self, name: &str) -> Option<&Column> {
        self.columns.iter().find(|c| c.name == name)
    }

    pub fn pk_columns(&self) -> Vec<&Column> {
        match &self.primary_key {
            Some(pk) => pk.iter().filter_map(|name| self.column(name)).collect(),
            None => vec![],
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Column {
    pub name: String,
    pub data_type: PgType,
    pub is_nullable: bool,
    pub has_default: bool,
    pub default_value: Option<String>,
    pub is_primary_key: bool,
    pub is_unique: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum PgType {
    Uuid,
    Text,
    Int4,
    Int8,
    Float8,
    Bool,
    Timestamp,
    Timestamptz,
    Date,
    Jsonb,
    Json,
    Bytea,
    Numeric,
    Serial,
    BigSerial,
    Other(String),
}

impl PgType {
    pub fn from_pg(type_name: &str) -> Self {
        match type_name {
            "uuid" => PgType::Uuid,
            "text" | "varchar" | "character varying" | "char" | "character" => PgType::Text,
            "int4" | "integer" | "int" => PgType::Int4,
            "int8" | "bigint" => PgType::Int8,
            "float8" | "double precision" => PgType::Float8,
            "bool" | "boolean" => PgType::Bool,
            "timestamp" | "timestamp without time zone" => PgType::Timestamp,
            "timestamptz" | "timestamp with time zone" => PgType::Timestamptz,
            "date" => PgType::Date,
            "jsonb" => PgType::Jsonb,
            "json" => PgType::Json,
            "bytea" => PgType::Bytea,
            "numeric" | "decimal" => PgType::Numeric,
            "serial" => PgType::Serial,
            "bigserial" => PgType::BigSerial,
            other => PgType::Other(other.to_string()),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ForeignKey {
    pub columns: Vec<String>,
    pub referenced_table: String,
    pub referenced_columns: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Index {
    pub name: String,
    pub columns: Vec<String>,
    pub is_unique: bool,
}
```

- [ ] **Step 2: Create schema module**

```rust
// engine/crates/garance-engine/src/schema/mod.rs
pub mod types;

pub use types::*;
```

- [ ] **Step 3: Update lib.rs**

```rust
// engine/crates/garance-engine/src/lib.rs
pub mod schema;

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}
```

- [ ] **Step 4: Write unit tests**

```rust
// engine/crates/garance-engine/tests/schema_types_test.rs
use garance_engine::schema::{PgType, Column, Table, ForeignKey, Schema};
use std::collections::HashMap;

#[test]
fn test_pg_type_from_pg() {
    assert_eq!(PgType::from_pg("uuid"), PgType::Uuid);
    assert_eq!(PgType::from_pg("text"), PgType::Text);
    assert_eq!(PgType::from_pg("varchar"), PgType::Text);
    assert_eq!(PgType::from_pg("character varying"), PgType::Text);
    assert_eq!(PgType::from_pg("integer"), PgType::Int4);
    assert_eq!(PgType::from_pg("boolean"), PgType::Bool);
    assert_eq!(PgType::from_pg("timestamp with time zone"), PgType::Timestamptz);
    assert_eq!(PgType::from_pg("jsonb"), PgType::Jsonb);
    assert_eq!(PgType::from_pg("custom_type"), PgType::Other("custom_type".into()));
}

#[test]
fn test_table_column_lookup() {
    let table = Table {
        name: "users".into(),
        schema: "public".into(),
        columns: vec![
            Column {
                name: "id".into(),
                data_type: PgType::Uuid,
                is_nullable: false,
                has_default: true,
                default_value: Some("gen_random_uuid()".into()),
                is_primary_key: true,
                is_unique: true,
            },
            Column {
                name: "email".into(),
                data_type: PgType::Text,
                is_nullable: false,
                has_default: false,
                default_value: None,
                is_primary_key: false,
                is_unique: true,
            },
        ],
        primary_key: Some(vec!["id".into()]),
        foreign_keys: vec![],
        indexes: vec![],
    };

    assert!(table.column("id").is_some());
    assert!(table.column("email").is_some());
    assert!(table.column("nonexistent").is_none());
    assert_eq!(table.pk_columns().len(), 1);
    assert_eq!(table.pk_columns()[0].name, "id");
}

#[test]
fn test_schema_serialization_roundtrip() {
    let mut tables = HashMap::new();
    tables.insert("users".into(), Table {
        name: "users".into(),
        schema: "public".into(),
        columns: vec![],
        primary_key: None,
        foreign_keys: vec![],
        indexes: vec![],
    });

    let schema = Schema { tables };
    let json = serde_json::to_string(&schema).unwrap();
    let deserialized: Schema = serde_json::from_str(&json).unwrap();
    assert_eq!(schema, deserialized);
}
```

- [ ] **Step 5: Run tests**

Run: `cd engine && cargo test -p garance-engine`
Expected: 3 tests pass.

- [ ] **Step 6: Commit**

```bash
git add engine/
git commit -m ":sparkles: feat(engine): add core schema types (Table, Column, PgType, ForeignKey)"
```

---

## Task 4: Schema Introspection — Read PG Schema

**Files:**
- Create: `engine/crates/garance-engine/src/schema/introspect.rs`
- Modify: `engine/crates/garance-engine/src/schema/mod.rs`
- Create: `engine/crates/garance-engine/tests/introspect_test.rs`

- [ ] **Step 1: Write the introspection module**

```rust
// engine/crates/garance-engine/src/schema/introspect.rs
use tokio_postgres::Client;
use std::collections::HashMap;
use tracing::info;

use super::types::*;

/// Introspect the PostgreSQL schema for user-defined tables.
/// Reads from information_schema and pg_catalog.
pub async fn introspect(client: &Client, schema_name: &str) -> Result<Schema, tokio_postgres::Error> {
    let tables = introspect_tables(client, schema_name).await?;
    info!(schema = schema_name, table_count = tables.len(), "schema introspected");
    Ok(Schema { tables })
}

async fn introspect_tables(client: &Client, schema_name: &str) -> Result<HashMap<String, Table>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT table_name FROM information_schema.tables
         WHERE table_schema = $1 AND table_type = 'BASE TABLE'
         ORDER BY table_name",
        &[&schema_name],
    ).await?;

    let mut tables = HashMap::new();
    for row in &rows {
        let table_name: &str = row.get("table_name");
        let columns = introspect_columns(client, schema_name, table_name).await?;
        let primary_key = introspect_primary_key(client, schema_name, table_name).await?;
        let foreign_keys = introspect_foreign_keys(client, schema_name, table_name).await?;
        let indexes = introspect_indexes(client, schema_name, table_name).await?;

        tables.insert(table_name.to_string(), Table {
            name: table_name.to_string(),
            schema: schema_name.to_string(),
            columns,
            primary_key,
            foreign_keys,
            indexes,
        });
    }

    Ok(tables)
}

async fn introspect_columns(client: &Client, schema_name: &str, table_name: &str) -> Result<Vec<Column>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT
            c.column_name,
            c.udt_name,
            c.is_nullable,
            c.column_default,
            EXISTS (
                SELECT 1 FROM information_schema.table_constraints tc
                JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
                WHERE tc.table_schema = $1 AND tc.table_name = $2
                AND kcu.column_name = c.column_name AND tc.constraint_type = 'PRIMARY KEY'
            ) as is_pk,
            EXISTS (
                SELECT 1 FROM information_schema.table_constraints tc
                JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
                WHERE tc.table_schema = $1 AND tc.table_name = $2
                AND kcu.column_name = c.column_name AND tc.constraint_type = 'UNIQUE'
            ) as is_unique
         FROM information_schema.columns c
         WHERE c.table_schema = $1 AND c.table_name = $2
         ORDER BY c.ordinal_position",
        &[&schema_name, &table_name],
    ).await?;

    Ok(rows.iter().map(|row| {
        let udt_name: &str = row.get("udt_name");
        let is_nullable: &str = row.get("is_nullable");
        let default_value: Option<&str> = row.get("column_default");

        Column {
            name: row.get::<_, &str>("column_name").to_string(),
            data_type: PgType::from_pg(udt_name),
            is_nullable: is_nullable == "YES",
            has_default: default_value.is_some(),
            default_value: default_value.map(String::from),
            is_primary_key: row.get("is_pk"),
            is_unique: row.get("is_unique"),
        }
    }).collect())
}

async fn introspect_primary_key(client: &Client, schema_name: &str, table_name: &str) -> Result<Option<Vec<String>>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT kcu.column_name
         FROM information_schema.table_constraints tc
         JOIN information_schema.key_column_usage kcu
           ON tc.constraint_name = kcu.constraint_name
           AND tc.table_schema = kcu.table_schema
         WHERE tc.table_schema = $1
           AND tc.table_name = $2
           AND tc.constraint_type = 'PRIMARY KEY'
         ORDER BY kcu.ordinal_position",
        &[&schema_name, &table_name],
    ).await?;

    if rows.is_empty() {
        return Ok(None);
    }

    Ok(Some(rows.iter().map(|r| r.get::<_, &str>("column_name").to_string()).collect()))
}

async fn introspect_foreign_keys(client: &Client, schema_name: &str, table_name: &str) -> Result<Vec<ForeignKey>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT
            tc.constraint_name,
            kcu.column_name,
            ccu.table_name AS referenced_table,
            ccu.column_name AS referenced_column
         FROM information_schema.table_constraints tc
         JOIN information_schema.key_column_usage kcu
           ON tc.constraint_name = kcu.constraint_name
           AND tc.table_schema = kcu.table_schema
         JOIN information_schema.constraint_column_usage ccu
           ON tc.constraint_name = ccu.constraint_name
           AND tc.table_schema = ccu.table_schema
         WHERE tc.table_schema = $1
           AND tc.table_name = $2
           AND tc.constraint_type = 'FOREIGN KEY'
         ORDER BY tc.constraint_name, kcu.ordinal_position",
        &[&schema_name, &table_name],
    ).await?;

    // Group by constraint name to correctly handle composite FKs
    // and multiple FKs to the same referenced table
    let mut fks_by_constraint: HashMap<String, ForeignKey> = HashMap::new();
    for row in &rows {
        let constraint: String = row.get::<_, &str>("constraint_name").to_string();
        let col: String = row.get::<_, &str>("column_name").to_string();
        let ref_table: String = row.get::<_, &str>("referenced_table").to_string();
        let ref_col: String = row.get::<_, &str>("referenced_column").to_string();

        let fk = fks_by_constraint.entry(constraint).or_insert_with(|| ForeignKey {
            columns: vec![],
            referenced_table: ref_table,
            referenced_columns: vec![],
        });
        if !fk.columns.contains(&col) {
            fk.columns.push(col);
        }
        if !fk.referenced_columns.contains(&ref_col) {
            fk.referenced_columns.push(ref_col);
        }
    }

    Ok(fks_by_constraint.into_values().collect())
}

async fn introspect_indexes(client: &Client, schema_name: &str, table_name: &str) -> Result<Vec<Index>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT
            i.relname as index_name,
            a.attname as column_name,
            ix.indisunique as is_unique
         FROM pg_index ix
         JOIN pg_class t ON t.oid = ix.indrelid
         JOIN pg_class i ON i.oid = ix.indexrelid
         JOIN pg_namespace n ON n.oid = t.relnamespace
         JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
         WHERE n.nspname = $1
           AND t.relname = $2
           AND NOT ix.indisprimary
         ORDER BY i.relname, a.attnum",
        &[&schema_name, &table_name],
    ).await?;

    let mut index_map: HashMap<String, Index> = HashMap::new();
    for row in &rows {
        let name: String = row.get::<_, &str>("index_name").to_string();
        let col: String = row.get::<_, &str>("column_name").to_string();
        let is_unique: bool = row.get("is_unique");

        let index = index_map.entry(name.clone()).or_insert_with(|| Index {
            name,
            columns: vec![],
            is_unique,
        });
        index.columns.push(col);
    }

    Ok(index_map.into_values().collect())
}
```

- [ ] **Step 2: Update schema/mod.rs**

```rust
// engine/crates/garance-engine/src/schema/mod.rs
pub mod types;
pub mod introspect;

pub use types::*;
pub use introspect::introspect;
```

- [ ] **Step 3: Write integration test**

```rust
// engine/crates/garance-engine/tests/introspect_test.rs
use garance_engine::schema::{introspect, PgType};
use testcontainers::runners::AsyncRunner;
use testcontainers_modules::postgres::Postgres;
use tokio_postgres::NoTls;

async fn setup_pg() -> (testcontainers::ContainerAsync<Postgres>, tokio_postgres::Client) {
    let container = Postgres::default().start().await.unwrap();
    let port = container.get_host_port_ipv4(5432).await.unwrap();
    let (client, connection) = tokio_postgres::connect(
        &format!("host=127.0.0.1 port={port} user=postgres password=postgres dbname=postgres"),
        NoTls,
    ).await.unwrap();
    tokio::spawn(async move { connection.await.unwrap(); });
    (container, client)
}

#[tokio::test]
async fn test_introspect_empty_schema() {
    let (_container, client) = setup_pg().await;
    let schema = introspect(&client, "public").await.unwrap();
    assert!(schema.tables.is_empty());
}

#[tokio::test]
async fn test_introspect_simple_table() {
    let (_container, client) = setup_pg().await;

    client.execute(
        "CREATE TABLE users (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            email text UNIQUE NOT NULL,
            name text NOT NULL,
            created_at timestamptz DEFAULT now()
        )", &[]
    ).await.unwrap();

    let schema = introspect(&client, "public").await.unwrap();
    assert_eq!(schema.tables.len(), 1);

    let users = schema.tables.get("users").unwrap();
    assert_eq!(users.columns.len(), 4);
    assert_eq!(users.primary_key, Some(vec!["id".into()]));

    let id_col = users.column("id").unwrap();
    assert_eq!(id_col.data_type, PgType::Uuid);
    assert!(id_col.is_primary_key);
    assert!(!id_col.is_nullable);
    assert!(id_col.has_default);

    let email_col = users.column("email").unwrap();
    assert_eq!(email_col.data_type, PgType::Text);
    assert!(email_col.is_unique);
    assert!(!email_col.is_nullable);
}

#[tokio::test]
async fn test_introspect_foreign_keys() {
    let (_container, client) = setup_pg().await;

    client.execute(
        "CREATE TABLE users (id uuid PRIMARY KEY DEFAULT gen_random_uuid())", &[]
    ).await.unwrap();

    client.execute(
        "CREATE TABLE posts (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            author_id uuid REFERENCES users(id),
            title text NOT NULL
        )", &[]
    ).await.unwrap();

    let schema = introspect(&client, "public").await.unwrap();
    let posts = schema.tables.get("posts").unwrap();

    assert_eq!(posts.foreign_keys.len(), 1);
    assert_eq!(posts.foreign_keys[0].referenced_table, "users");
    assert_eq!(posts.foreign_keys[0].columns, vec!["author_id"]);
    assert_eq!(posts.foreign_keys[0].referenced_columns, vec!["id"]);
}

#[tokio::test]
async fn test_introspect_indexes() {
    let (_container, client) = setup_pg().await;

    client.execute(
        "CREATE TABLE products (
            id serial PRIMARY KEY,
            name text NOT NULL,
            category text NOT NULL
        )", &[]
    ).await.unwrap();
    client.execute("CREATE INDEX idx_products_category ON products(category)", &[]).await.unwrap();

    let schema = introspect(&client, "public").await.unwrap();
    let products = schema.tables.get("products").unwrap();

    let cat_idx = products.indexes.iter().find(|i| i.name == "idx_products_category");
    assert!(cat_idx.is_some());
    assert_eq!(cat_idx.unwrap().columns, vec!["category"]);
    assert!(!cat_idx.unwrap().is_unique);
}
```

- [ ] **Step 4: Run tests**

Run: `cd engine && cargo test -p garance-engine`
Expected: All tests pass (4 integration + 3 unit from Task 3).

- [ ] **Step 5: Commit**

```bash
git add engine/
git commit -m ":sparkles: feat(engine): add PostgreSQL schema introspection"
```

---

## Task 5: Query Builder — REST Params to SQL

**Files:**
- Create: `engine/crates/garance-engine/src/query/mod.rs`
- Create: `engine/crates/garance-engine/src/query/filter.rs`
- Create: `engine/crates/garance-engine/src/query/builder.rs`
- Create: `engine/crates/garance-engine/src/query/error.rs`
- Modify: `engine/crates/garance-engine/src/lib.rs`
- Create: `engine/crates/garance-engine/tests/query_test.rs`

- [ ] **Step 1: Write query error types**

```rust
// engine/crates/garance-engine/src/query/error.rs
use thiserror::Error;

#[derive(Error, Debug)]
pub enum QueryError {
    #[error("unknown table: {0}")]
    UnknownTable(String),

    #[error("unknown column '{column}' on table '{table}'")]
    UnknownColumn { table: String, column: String },

    #[error("invalid filter operator: {0}")]
    InvalidOperator(String),

    #[error("invalid filter value for column '{column}': {reason}")]
    InvalidValue { column: String, reason: String },

    #[error("database error: {0}")]
    Database(#[from] tokio_postgres::Error),
}
```

- [ ] **Step 2: Write filter parser**

Parses `age=gte.18&city=eq.Paris` into structured filters.

```rust
// engine/crates/garance-engine/src/query/filter.rs
use super::error::QueryError;

#[derive(Debug, Clone, PartialEq)]
pub enum Operator {
    Eq,
    Neq,
    Gt,
    Gte,
    Lt,
    Lte,
    Like,
    Ilike,
    Is,
    In,
}

impl Operator {
    pub fn from_str(s: &str) -> Result<Self, QueryError> {
        match s {
            "eq" => Ok(Operator::Eq),
            "neq" => Ok(Operator::Neq),
            "gt" => Ok(Operator::Gt),
            "gte" => Ok(Operator::Gte),
            "lt" => Ok(Operator::Lt),
            "lte" => Ok(Operator::Lte),
            "like" => Ok(Operator::Like),
            "ilike" => Ok(Operator::Ilike),
            "is" => Ok(Operator::Is),
            "in" => Ok(Operator::In),
            other => Err(QueryError::InvalidOperator(other.into())),
        }
    }

    pub fn to_sql(&self) -> &'static str {
        match self {
            Operator::Eq => "=",
            Operator::Neq => "!=",
            Operator::Gt => ">",
            Operator::Gte => ">=",
            Operator::Lt => "<",
            Operator::Lte => "<=",
            Operator::Like => "LIKE",
            Operator::Ilike => "ILIKE",
            Operator::Is => "IS",
            Operator::In => "IN",
        }
    }
}

#[derive(Debug, Clone, PartialEq)]
pub struct Filter {
    pub column: String,
    pub operator: Operator,
    pub value: String,
}

#[derive(Debug, Clone, PartialEq)]
pub enum SortDirection {
    Asc,
    Desc,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Sort {
    pub column: String,
    pub direction: SortDirection,
}

#[derive(Debug, Clone, Default)]
pub struct QueryParams {
    pub select: Option<Vec<String>>,
    pub filters: Vec<Filter>,
    pub order: Vec<Sort>,
    pub limit: Option<i64>,
    pub offset: Option<i64>,
}

/// Parse query string params into QueryParams.
/// Reserved params: select, order, limit, offset. Everything else is a filter.
pub fn parse_query_params(params: &[(String, String)]) -> Result<QueryParams, QueryError> {
    let mut qp = QueryParams::default();

    for (key, value) in params {
        match key.as_str() {
            "select" => {
                qp.select = Some(value.split(',').map(|s| s.trim().to_string()).collect());
            }
            "order" => {
                for part in value.split(',') {
                    let parts: Vec<&str> = part.trim().split('.').collect();
                    let column = parts[0].to_string();
                    let direction = match parts.get(1) {
                        Some(&"desc") => SortDirection::Desc,
                        _ => SortDirection::Asc,
                    };
                    qp.order.push(Sort { column, direction });
                }
            }
            "limit" => {
                qp.limit = Some(value.parse().map_err(|_| QueryError::InvalidValue {
                    column: "limit".into(),
                    reason: "must be an integer".into(),
                })?);
            }
            "offset" => {
                qp.offset = Some(value.parse().map_err(|_| QueryError::InvalidValue {
                    column: "offset".into(),
                    reason: "must be an integer".into(),
                })?);
            }
            column => {
                // Format: operator.value (e.g., "gte.18")
                let dot_pos = value.find('.').ok_or_else(|| QueryError::InvalidValue {
                    column: column.into(),
                    reason: "filter must be in format operator.value (e.g., eq.Paris)".into(),
                })?;
                let (op_str, val) = value.split_at(dot_pos);
                let val = &val[1..]; // skip the dot
                let operator = Operator::from_str(op_str)?;

                qp.filters.push(Filter {
                    column: column.to_string(),
                    operator,
                    value: val.to_string(),
                });
            }
        }
    }

    Ok(qp)
}
```

- [ ] **Step 3: Write SQL builder**

```rust
// engine/crates/garance-engine/src/query/builder.rs
use crate::schema::types::Table;
use super::filter::{QueryParams, SortDirection, Operator};
use super::error::QueryError;

#[derive(Debug)]
pub struct SqlQuery {
    pub sql: String,
    pub params: Vec<String>,
}

pub fn build_select(table: &Table, qp: &QueryParams) -> Result<SqlQuery, QueryError> {
    let mut params: Vec<String> = vec![];
    let mut param_idx = 1;

    // SELECT
    let columns = match &qp.select {
        Some(cols) => {
            for col in cols {
                if table.column(col).is_none() {
                    return Err(QueryError::UnknownColumn {
                        table: table.name.clone(),
                        column: col.clone(),
                    });
                }
            }
            cols.iter().map(|c| format!("\"{}\"", c)).collect::<Vec<_>>().join(", ")
        }
        None => "*".to_string(),
    };

    let mut sql = format!("SELECT {} FROM \"{}\"", columns, table.name);

    // WHERE
    if !qp.filters.is_empty() {
        let mut conditions = vec![];
        for filter in &qp.filters {
            if table.column(&filter.column).is_none() {
                return Err(QueryError::UnknownColumn {
                    table: table.name.clone(),
                    column: filter.column.clone(),
                });
            }

            match filter.operator {
                Operator::Is => {
                    let val = match filter.value.to_lowercase().as_str() {
                        "null" => "NULL",
                        "true" => "TRUE",
                        "false" => "FALSE",
                        _ => return Err(QueryError::InvalidValue {
                            column: filter.column.clone(),
                            reason: "IS only supports null, true, false".into(),
                        }),
                    };
                    conditions.push(format!("\"{}\" IS {}", filter.column, val));
                }
                Operator::In => {
                    let values: Vec<&str> = filter.value.split(',').collect();
                    let placeholders: Vec<String> = values.iter().map(|v| {
                        params.push(v.trim().to_string());
                        let p = format!("${}", param_idx);
                        param_idx += 1;
                        p
                    }).collect();
                    conditions.push(format!("\"{}\" IN ({})", filter.column, placeholders.join(", ")));
                }
                _ => {
                    // Cast column to text for comparison — allows string params to match any PG type
                    params.push(filter.value.clone());
                    conditions.push(format!("\"{}\"::text {} ${}", filter.column, filter.operator.to_sql(), param_idx));
                    param_idx += 1;
                }
            }
        }
        sql.push_str(&format!(" WHERE {}", conditions.join(" AND ")));
    }

    // ORDER BY
    if !qp.order.is_empty() {
        let order_parts: Vec<String> = qp.order.iter().map(|s| {
            let dir = match s.direction {
                SortDirection::Asc => "ASC",
                SortDirection::Desc => "DESC",
            };
            format!("\"{}\" {}", s.column, dir)
        }).collect();
        sql.push_str(&format!(" ORDER BY {}", order_parts.join(", ")));
    }

    // LIMIT / OFFSET
    if let Some(limit) = qp.limit {
        sql.push_str(&format!(" LIMIT {}", limit));
    }
    if let Some(offset) = qp.offset {
        sql.push_str(&format!(" OFFSET {}", offset));
    }

    Ok(SqlQuery { sql, params })
}

pub fn build_select_by_id(table: &Table, id_value: &str) -> Result<SqlQuery, QueryError> {
    let pk = table.primary_key.as_ref().ok_or_else(|| QueryError::UnknownColumn {
        table: table.name.clone(),
        column: "(primary key)".into(),
    })?;

    let pk_col = &pk[0]; // Support single-column PK for now
    // Cast to text so string param works with any PK type (uuid, serial, etc.)
    let sql = format!("SELECT * FROM \"{}\" WHERE \"{}\"::text = $1", table.name, pk_col);
    Ok(SqlQuery { sql, params: vec![id_value.to_string()] })
}

pub fn build_insert(table: &Table, columns: &[String]) -> Result<SqlQuery, QueryError> {
    for col in columns {
        if table.column(col).is_none() {
            return Err(QueryError::UnknownColumn {
                table: table.name.clone(),
                column: col.clone(),
            });
        }
    }

    let col_names: Vec<String> = columns.iter().map(|c| format!("\"{}\"", c)).collect();
    let placeholders: Vec<String> = (1..=columns.len()).map(|i| format!("${}", i)).collect();

    let sql = format!(
        "INSERT INTO \"{}\" ({}) VALUES ({}) RETURNING *",
        table.name,
        col_names.join(", "),
        placeholders.join(", ")
    );

    Ok(SqlQuery { sql, params: vec![] })
}

pub fn build_update(table: &Table, id_value: &str, columns: &[String]) -> Result<SqlQuery, QueryError> {
    let pk = table.primary_key.as_ref().ok_or_else(|| QueryError::UnknownColumn {
        table: table.name.clone(),
        column: "(primary key)".into(),
    })?;

    for col in columns {
        if table.column(col).is_none() {
            return Err(QueryError::UnknownColumn {
                table: table.name.clone(),
                column: col.clone(),
            });
        }
    }

    let set_clauses: Vec<String> = columns.iter().enumerate()
        .map(|(i, col)| format!("\"{}\" = ${}", col, i + 1))
        .collect();

    let pk_col = &pk[0];
    let sql = format!(
        "UPDATE \"{}\" SET {} WHERE \"{}\"::text = ${} RETURNING *",
        table.name,
        set_clauses.join(", "),
        pk_col,
        columns.len() + 1
    );

    Ok(SqlQuery { sql, params: vec![id_value.to_string()] })
}

pub fn build_delete(table: &Table, id_value: &str) -> Result<SqlQuery, QueryError> {
    let pk = table.primary_key.as_ref().ok_or_else(|| QueryError::UnknownColumn {
        table: table.name.clone(),
        column: "(primary key)".into(),
    })?;

    let pk_col = &pk[0];
    let sql = format!("DELETE FROM \"{}\" WHERE \"{}\"::text = $1", table.name, pk_col);
    Ok(SqlQuery { sql, params: vec![id_value.to_string()] })
}
```

- [ ] **Step 4: Create query module**

```rust
// engine/crates/garance-engine/src/query/mod.rs
pub mod builder;
pub mod error;
pub mod filter;

pub use builder::*;
pub use error::QueryError;
pub use filter::*;
```

- [ ] **Step 5: Update lib.rs**

```rust
// engine/crates/garance-engine/src/lib.rs
pub mod schema;
pub mod query;

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}
```

- [ ] **Step 6: Write unit tests**

```rust
// engine/crates/garance-engine/tests/query_test.rs
use garance_engine::query::filter::*;
use garance_engine::query::builder::*;
use garance_engine::schema::types::*;

fn test_table() -> Table {
    Table {
        name: "users".into(),
        schema: "public".into(),
        columns: vec![
            Column { name: "id".into(), data_type: PgType::Uuid, is_nullable: false, has_default: true, default_value: None, is_primary_key: true, is_unique: true },
            Column { name: "name".into(), data_type: PgType::Text, is_nullable: false, has_default: false, default_value: None, is_primary_key: false, is_unique: false },
            Column { name: "age".into(), data_type: PgType::Int4, is_nullable: true, has_default: false, default_value: None, is_primary_key: false, is_unique: false },
            Column { name: "email".into(), data_type: PgType::Text, is_nullable: false, has_default: false, default_value: None, is_primary_key: false, is_unique: true },
        ],
        primary_key: Some(vec!["id".into()]),
        foreign_keys: vec![],
        indexes: vec![],
    }
}

#[test]
fn test_parse_simple_filter() {
    let params = vec![
        ("age".into(), "gte.18".into()),
        ("name".into(), "eq.Alice".into()),
    ];
    let qp = parse_query_params(&params).unwrap();
    assert_eq!(qp.filters.len(), 2);
    assert_eq!(qp.filters[0].column, "age");
    assert_eq!(qp.filters[0].operator, Operator::Gte);
    assert_eq!(qp.filters[0].value, "18");
}

#[test]
fn test_parse_select_and_order() {
    let params = vec![
        ("select".into(), "id,name,email".into()),
        ("order".into(), "name.asc,age.desc".into()),
        ("limit".into(), "20".into()),
    ];
    let qp = parse_query_params(&params).unwrap();
    assert_eq!(qp.select, Some(vec!["id".into(), "name".into(), "email".into()]));
    assert_eq!(qp.order.len(), 2);
    assert_eq!(qp.order[0].column, "name");
    assert_eq!(qp.order[0].direction, SortDirection::Asc);
    assert_eq!(qp.order[1].direction, SortDirection::Desc);
    assert_eq!(qp.limit, Some(20));
}

#[test]
fn test_build_select_simple() {
    let table = test_table();
    let qp = QueryParams::default();
    let result = build_select(&table, &qp).unwrap();
    assert_eq!(result.sql, "SELECT * FROM \"users\"");
    assert!(result.params.is_empty());
}

#[test]
fn test_build_select_with_filters() {
    let table = test_table();
    let qp = QueryParams {
        filters: vec![Filter { column: "age".into(), operator: Operator::Gte, value: "18".into() }],
        limit: Some(10),
        ..Default::default()
    };
    let result = build_select(&table, &qp).unwrap();
    assert_eq!(result.sql, "SELECT * FROM \"users\" WHERE \"age\"::text >= $1 LIMIT 10");
    assert_eq!(result.params, vec!["18"]);
}

#[test]
fn test_build_select_unknown_column_rejected() {
    let table = test_table();
    let qp = QueryParams {
        select: Some(vec!["nonexistent".into()]),
        ..Default::default()
    };
    assert!(build_select(&table, &qp).is_err());
}

#[test]
fn test_build_select_by_id() {
    let table = test_table();
    let result = build_select_by_id(&table, "abc-123").unwrap();
    assert_eq!(result.sql, "SELECT * FROM \"users\" WHERE \"id\"::text = $1");
    assert_eq!(result.params, vec!["abc-123"]);
}

#[test]
fn test_build_insert() {
    let table = test_table();
    let cols = vec!["name".into(), "email".into()];
    let result = build_insert(&table, &cols).unwrap();
    assert_eq!(result.sql, "INSERT INTO \"users\" (\"name\", \"email\") VALUES ($1, $2) RETURNING *");
}

#[test]
fn test_build_update() {
    let table = test_table();
    let cols = vec!["name".into()];
    let result = build_update(&table, "abc-123", &cols).unwrap();
    assert_eq!(result.sql, "UPDATE \"users\" SET \"name\" = $1 WHERE \"id\"::text = $2 RETURNING *");
}

#[test]
fn test_build_delete() {
    let table = test_table();
    let result = build_delete(&table, "abc-123").unwrap();
    assert_eq!(result.sql, "DELETE FROM \"users\" WHERE \"id\"::text = $1");
}
```

- [ ] **Step 7: Run tests**

Run: `cd engine && cargo test -p garance-engine`
Expected: All tests pass (unit + integration).

- [ ] **Step 8: Commit**

```bash
git add engine/
git commit -m ":sparkles: feat(engine): add query builder with filter parsing and SQL generation"
```

---

## Task 6: HTTP API — axum REST Endpoints

**Files:**
- Create: `engine/crates/garance-engine/src/api/mod.rs`
- Create: `engine/crates/garance-engine/src/api/routes.rs`
- Create: `engine/crates/garance-engine/src/api/error.rs`
- Create: `engine/crates/garance-engine/src/api/state.rs`
- Modify: `engine/crates/garance-engine/src/lib.rs`
- Modify: `engine/crates/garance-engine/src/main.rs`
- Create: `engine/crates/garance-engine/tests/api_test.rs`

- [ ] **Step 1: Write API error responses**

Follows the error format from spec section 13.

```rust
// engine/crates/garance-engine/src/api/error.rs
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::Json;
use serde::Serialize;

use crate::query::QueryError;

#[derive(Serialize)]
pub struct ApiError {
    pub error: ApiErrorBody,
}

#[derive(Serialize)]
pub struct ApiErrorBody {
    pub code: String,
    pub message: String,
    pub status: u16,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<serde_json::Value>,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let status = StatusCode::from_u16(self.error.status).unwrap_or(StatusCode::INTERNAL_SERVER_ERROR);
        (status, Json(self)).into_response()
    }
}

impl From<QueryError> for ApiError {
    fn from(err: QueryError) -> Self {
        match &err {
            QueryError::UnknownTable(_) => ApiError {
                error: ApiErrorBody {
                    code: "NOT_FOUND".into(),
                    message: err.to_string(),
                    status: 404,
                    details: None,
                },
            },
            QueryError::UnknownColumn { .. } => ApiError {
                error: ApiErrorBody {
                    code: "VALIDATION_ERROR".into(),
                    message: err.to_string(),
                    status: 400,
                    details: None,
                },
            },
            QueryError::InvalidOperator(_) | QueryError::InvalidValue { .. } => ApiError {
                error: ApiErrorBody {
                    code: "VALIDATION_ERROR".into(),
                    message: err.to_string(),
                    status: 400,
                    details: None,
                },
            },
            QueryError::Database(_) => ApiError {
                error: ApiErrorBody {
                    code: "INTERNAL_ERROR".into(),
                    message: "internal database error".into(),
                    status: 500,
                    details: None,
                },
            },
        }
    }
}

impl From<tokio_postgres::Error> for ApiError {
    fn from(_err: tokio_postgres::Error) -> Self {
        ApiError {
            error: ApiErrorBody {
                code: "INTERNAL_ERROR".into(),
                message: "internal database error".into(),
                status: 500,
                details: None,
            },
        }
    }
}
```

- [ ] **Step 2: Write app state**

```rust
// engine/crates/garance-engine/src/api/state.rs
use std::sync::Arc;
use tokio::sync::RwLock;

use garance_pooler::GarancePool;
use crate::schema::types::Schema;

#[derive(Clone)]
pub struct AppState {
    pub pool: Arc<GarancePool>,
    pub schema: Arc<RwLock<Schema>>,
}
```

- [ ] **Step 3: Write route handlers**

```rust
// engine/crates/garance-engine/src/api/routes.rs
use axum::extract::{Path, Query, State};
use axum::http::StatusCode;
use axum::response::IntoResponse;
use axum::Json;
use serde_json::{json, Value, Map};
use std::collections::HashMap;

use super::error::ApiError;
use super::state::AppState;
use crate::query::filter::parse_query_params;
use crate::query::builder::*;

/// GET /api/v1/:table — list rows with filtering
pub async fn list_rows(
    State(state): State<AppState>,
    Path(table_name): Path<String>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;

    let param_vec: Vec<(String, String)> = params.into_iter().collect();
    let qp = parse_query_params(&param_vec)?;
    let sql_query = build_select(table, &qp)?;

    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody {
            code: "INTERNAL_ERROR".into(),
            message: e.to_string(),
            status: 500,
            details: None,
        },
    })?;

    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
        sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

    let rows = client.query(&sql_query.sql, &params_refs).await?;
    let results: Vec<Value> = rows.iter().map(row_to_json).collect();

    Ok(Json(results))
}

/// GET /api/v1/:table/:id — get single row
pub async fn get_row(
    State(state): State<AppState>,
    Path((table_name, id)): Path<(String, String)>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;

    let sql_query = build_select_by_id(table, &id)?;

    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody {
            code: "INTERNAL_ERROR".into(),
            message: e.to_string(),
            status: 500,
            details: None,
        },
    })?;

    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
        sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

    let row = client.query_opt(&sql_query.sql, &params_refs).await?;

    match row {
        Some(row) => Ok(Json(row_to_json(&row))),
        None => Err(ApiError {
            error: super::error::ApiErrorBody {
                code: "NOT_FOUND".into(),
                message: format!("{} with id '{}' not found", table_name, id),
                status: 404,
                details: None,
            },
        }),
    }
}

/// POST /api/v1/:table — insert row
pub async fn insert_row(
    State(state): State<AppState>,
    Path(table_name): Path<String>,
    Json(body): Json<Map<String, Value>>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;

    let columns: Vec<String> = body.keys().cloned().collect();
    let values: Vec<String> = body.values().map(|v| match v {
        Value::String(s) => s.clone(),
        other => other.to_string(),
    }).collect();

    let sql_query = build_insert(table, &columns)?;

    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody {
            code: "INTERNAL_ERROR".into(),
            message: e.to_string(),
            status: 500,
            details: None,
        },
    })?;

    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
        values.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

    let row = client.query_one(&sql_query.sql, &params_refs).await?;

    Ok((StatusCode::CREATED, Json(row_to_json(&row))))
}

/// PATCH /api/v1/:table/:id — update row
pub async fn update_row(
    State(state): State<AppState>,
    Path((table_name, id)): Path<(String, String)>,
    Json(body): Json<Map<String, Value>>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;

    let columns: Vec<String> = body.keys().cloned().collect();
    let values: Vec<String> = body.values().map(|v| match v {
        Value::String(s) => s.clone(),
        other => other.to_string(),
    }).collect();

    let sql_query = build_update(table, &id, &columns)?;

    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody {
            code: "INTERNAL_ERROR".into(),
            message: e.to_string(),
            status: 500,
            details: None,
        },
    })?;

    let mut all_params: Vec<String> = values;
    all_params.push(id.clone());
    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
        all_params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

    let row = client.query_opt(&sql_query.sql, &params_refs).await?;

    match row {
        Some(row) => Ok(Json(row_to_json(&row))),
        None => Err(ApiError {
            error: super::error::ApiErrorBody {
                code: "NOT_FOUND".into(),
                message: format!("{} with id '{}' not found", table_name, id),
                status: 404,
                details: None,
            },
        }),
    }
}

/// DELETE /api/v1/:table/:id — delete row
pub async fn delete_row(
    State(state): State<AppState>,
    Path((table_name, id)): Path<(String, String)>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;

    let sql_query = build_delete(table, &id)?;

    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody {
            code: "INTERNAL_ERROR".into(),
            message: e.to_string(),
            status: 500,
            details: None,
        },
    })?;

    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
        sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

    let affected = client.execute(&sql_query.sql, &params_refs).await?;

    if affected == 0 {
        return Err(ApiError {
            error: super::error::ApiErrorBody {
                code: "NOT_FOUND".into(),
                message: format!("{} with id '{}' not found", table_name, id),
                status: 404,
                details: None,
            },
        });
    }

    Ok(StatusCode::NO_CONTENT)
}

/// Convert a PG row to a JSON object with type-aware serialization.
fn row_to_json(row: &tokio_postgres::Row) -> Value {
    use tokio_postgres::types::Type;

    let mut obj = Map::new();
    for (i, col) in row.columns().iter().enumerate() {
        let value = match *col.type_() {
            Type::BOOL => row.get::<_, Option<bool>>(i).map(Value::Bool).unwrap_or(Value::Null),
            Type::INT2 => row.get::<_, Option<i16>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::INT4 => row.get::<_, Option<i32>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::INT8 => row.get::<_, Option<i64>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::FLOAT4 => row.get::<_, Option<f32>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::FLOAT8 => row.get::<_, Option<f64>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::UUID => row.get::<_, Option<uuid::Uuid>>(i).map(|v| Value::String(v.to_string())).unwrap_or(Value::Null),
            Type::JSON | Type::JSONB => row.get::<_, Option<serde_json::Value>>(i).unwrap_or(Value::Null),
            _ => {
                // Fallback: try as text
                row.get::<_, Option<&str>>(i)
                    .map(|v| Value::String(v.to_string()))
                    .unwrap_or(Value::Null)
            }
        };
        obj.insert(col.name().to_string(), value);
    }
    Value::Object(obj)
}
```

`row_to_json` uses type-aware serialization based on `tokio_postgres::types::Type`.

- [ ] **Step 4: Create API module and router**

```rust
// engine/crates/garance-engine/src/api/mod.rs
pub mod error;
pub mod routes;
pub mod state;

use axum::{routing::{get, post, patch, delete}, Router};
use state::AppState;

pub fn router(state: AppState) -> Router {
    Router::new()
        .route("/api/v1/{table}", get(routes::list_rows).post(routes::insert_row))
        .route("/api/v1/{table}/{id}", get(routes::get_row).patch(routes::update_row).delete(routes::delete_row))
        .route("/health", get(|| async { "ok" }))
        .with_state(state)
}
```

- [ ] **Step 5: Update lib.rs and main.rs**

```rust
// engine/crates/garance-engine/src/lib.rs
pub mod schema;
pub mod query;
pub mod api;

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}
```

```rust
// engine/crates/garance-engine/src/main.rs
use std::sync::Arc;
use tokio::sync::RwLock;
use tracing::info;
use tracing_subscriber::EnvFilter;

use garance_engine::api;
use garance_engine::schema;
use garance_pooler::{GarancePool, PoolConfig};

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt()
        .with_env_filter(EnvFilter::from_default_env().add_directive("garance=info".parse().unwrap()))
        .json()
        .init();

    let database_url = std::env::var("DATABASE_URL")
        .unwrap_or_else(|_| "postgresql://postgres:postgres@localhost:5432/postgres".into());

    let config = PoolConfig::from_url(&database_url).expect("invalid DATABASE_URL");
    let pool = GarancePool::new(&config).expect("failed to create pool");

    // Introspect schema on startup
    let client = pool.get().await.expect("failed to connect to database");
    let db_schema = schema::introspect(&client, "public").await.expect("failed to introspect schema");
    drop(client);

    info!(tables = db_schema.tables.len(), "schema introspected");

    let state = api::state::AppState {
        pool: Arc::new(pool),
        schema: Arc::new(RwLock::new(db_schema)),
    };

    let app = api::router(state);

    let addr = std::env::var("LISTEN_ADDR").unwrap_or_else(|_| "0.0.0.0:4000".into());
    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    info!(%addr, "garance engine started");
    axum::serve(listener, app).await.unwrap();
}
```

- [ ] **Step 6: Write integration test**

```rust
// engine/crates/garance-engine/tests/api_test.rs
use std::sync::Arc;
use tokio::sync::RwLock;
use axum_test::TestServer;
use serde_json::json;
use testcontainers::runners::AsyncRunner;
use testcontainers_modules::postgres::Postgres;
use tokio_postgres::NoTls;

use garance_engine::api;
use garance_engine::schema;
use garance_pooler::{GarancePool, PoolConfig};

async fn setup() -> (testcontainers::ContainerAsync<Postgres>, TestServer) {
    let container = Postgres::default().start().await.unwrap();
    let port = container.get_host_port_ipv4(5432).await.unwrap();

    let config = PoolConfig {
        host: "127.0.0.1".into(),
        port,
        user: "postgres".into(),
        password: "postgres".into(),
        dbname: "postgres".into(),
        max_size: 4,
    };

    let pool = GarancePool::new(&config).unwrap();

    // Create test table
    let client = pool.get().await.unwrap();
    client.execute(
        "CREATE TABLE users (
            id serial PRIMARY KEY,
            name text NOT NULL,
            email text UNIQUE NOT NULL
        )", &[]
    ).await.unwrap();
    drop(client);

    // Introspect
    let client = pool.get().await.unwrap();
    let db_schema = schema::introspect(&client, "public").await.unwrap();
    drop(client);

    let state = api::state::AppState {
        pool: Arc::new(pool),
        schema: Arc::new(RwLock::new(db_schema)),
    };

    let app = api::router(state);
    let server = TestServer::new(app).unwrap();
    (container, server)
}

#[tokio::test]
async fn test_health_check() {
    let (_container, server) = setup().await;
    let response = server.get("/health").await;
    response.assert_status_ok();
    response.assert_text("ok");
}

#[tokio::test]
async fn test_list_empty_table() {
    let (_container, server) = setup().await;
    let response = server.get("/api/v1/users").await;
    response.assert_status_ok();
    let body: Vec<serde_json::Value> = response.json();
    assert!(body.is_empty());
}

#[tokio::test]
async fn test_insert_and_get() {
    let (_container, server) = setup().await;

    // Insert
    let response = server.post("/api/v1/users")
        .json(&json!({"name": "Alice", "email": "alice@example.fr"}))
        .await;
    response.assert_status(axum::http::StatusCode::CREATED);
    let body: serde_json::Value = response.json();
    assert_eq!(body["name"], "Alice");
    assert_eq!(body["email"], "alice@example.fr");

    let id = body["id"].as_str().unwrap();

    // Get by id
    let response = server.get(&format!("/api/v1/users/{}", id)).await;
    response.assert_status_ok();
    let body: serde_json::Value = response.json();
    assert_eq!(body["name"], "Alice");
}

#[tokio::test]
async fn test_update() {
    let (_container, server) = setup().await;

    let response = server.post("/api/v1/users")
        .json(&json!({"name": "Bob", "email": "bob@example.fr"}))
        .await;
    let body: serde_json::Value = response.json();
    let id = body["id"].as_str().unwrap();

    let response = server.patch(&format!("/api/v1/users/{}", id))
        .json(&json!({"name": "Robert"}))
        .await;
    response.assert_status_ok();
    let body: serde_json::Value = response.json();
    assert_eq!(body["name"], "Robert");
}

#[tokio::test]
async fn test_delete() {
    let (_container, server) = setup().await;

    let response = server.post("/api/v1/users")
        .json(&json!({"name": "Charlie", "email": "charlie@example.fr"}))
        .await;
    let body: serde_json::Value = response.json();
    let id = body["id"].as_str().unwrap();

    let response = server.delete(&format!("/api/v1/users/{}", id)).await;
    response.assert_status(axum::http::StatusCode::NO_CONTENT);

    let response = server.get(&format!("/api/v1/users/{}", id)).await;
    response.assert_status(axum::http::StatusCode::NOT_FOUND);
}

#[tokio::test]
async fn test_unknown_table_returns_404() {
    let (_container, server) = setup().await;
    let response = server.get("/api/v1/nonexistent").await;
    response.assert_status(axum::http::StatusCode::NOT_FOUND);
    let body: serde_json::Value = response.json();
    assert_eq!(body["error"]["code"], "NOT_FOUND");
}

#[tokio::test]
async fn test_filter_params() {
    let (_container, server) = setup().await;

    server.post("/api/v1/users").json(&json!({"name": "Alice", "email": "a@x.fr"})).await;
    server.post("/api/v1/users").json(&json!({"name": "Bob", "email": "b@x.fr"})).await;

    let response = server.get("/api/v1/users?name=eq.Alice").await;
    response.assert_status_ok();
    let body: Vec<serde_json::Value> = response.json();
    assert_eq!(body.len(), 1);
    assert_eq!(body[0]["name"], "Alice");
}
```

- [ ] **Step 7: Run tests**

Run: `cd engine && cargo test -p garance-engine`
Expected: All tests pass.

- [ ] **Step 8: Commit**

```bash
git add engine/
git commit -m ":sparkles: feat(engine): add HTTP REST API with CRUD endpoints"
```

---

## Task 7: Schema JSON Reader

**Files:**
- Create: `engine/crates/garance-engine/src/schema/json_schema.rs`
- Modify: `engine/crates/garance-engine/src/schema/mod.rs`
- Create: `engine/crates/garance-engine/tests/json_schema_test.rs`

- [ ] **Step 1: Define the JSON schema format**

This is the intermediate format generated by `@garance/schema` from `garance.schema.ts`.

```rust
// engine/crates/garance-engine/src/schema/json_schema.rs
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// The format of garance.schema.json — compiled from garance.schema.ts by @garance/schema.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceSchema {
    pub version: u32,
    pub tables: HashMap<String, GaranceTable>,
    #[serde(default)]
    pub storage: HashMap<String, GaranceBucket>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceTable {
    pub columns: HashMap<String, GaranceColumn>,
    #[serde(default)]
    pub relations: HashMap<String, GaranceRelation>,
    #[serde(default)]
    pub access: Option<GaranceAccess>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceColumn {
    #[serde(rename = "type")]
    pub col_type: String,
    #[serde(default)]
    pub primary_key: bool,
    #[serde(default)]
    pub unique: bool,
    #[serde(default)]
    pub nullable: bool,
    #[serde(default)]
    pub default: Option<String>,
    #[serde(default)]
    pub references: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceRelation {
    #[serde(rename = "type")]
    pub rel_type: String, // "hasMany", "hasOne", "belongsTo"
    pub table: String,
    pub foreign_key: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceAccess {
    #[serde(default)]
    pub read: Option<AccessRule>,
    #[serde(default)]
    pub write: Option<AccessRule>,
    #[serde(default)]
    pub delete: Option<AccessRule>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum AccessRule {
    Public(String), // "public", "authenticated"
    Conditions(Vec<AccessCondition>),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AccessCondition {
    #[serde(rename = "type")]
    pub condition_type: String, // "isOwner", "where", "isAuthenticated"
    #[serde(default)]
    pub column: Option<String>,
    #[serde(default)]
    pub filters: Option<HashMap<String, serde_json::Value>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceBucket {
    #[serde(default)]
    pub max_file_size: Option<String>,
    #[serde(default)]
    pub allowed_mime_types: Option<Vec<String>>,
    #[serde(default)]
    pub access: Option<GaranceAccess>,
}

/// Load a garance.schema.json file.
pub fn load_schema(path: &std::path::Path) -> Result<GaranceSchema, SchemaLoadError> {
    let content = std::fs::read_to_string(path)
        .map_err(|e| SchemaLoadError::Io(e.to_string()))?;
    let schema: GaranceSchema = serde_json::from_str(&content)
        .map_err(|e| SchemaLoadError::Parse(e.to_string()))?;
    Ok(schema)
}

/// Load from a JSON string (for testing).
pub fn load_schema_from_str(json: &str) -> Result<GaranceSchema, SchemaLoadError> {
    serde_json::from_str(json).map_err(|e| SchemaLoadError::Parse(e.to_string()))
}

#[derive(Debug, thiserror::Error)]
pub enum SchemaLoadError {
    #[error("failed to read schema file: {0}")]
    Io(String),
    #[error("failed to parse schema JSON: {0}")]
    Parse(String),
}
```

- [ ] **Step 2: Update schema/mod.rs**

```rust
// engine/crates/garance-engine/src/schema/mod.rs
pub mod types;
pub mod introspect;
pub mod json_schema;

pub use types::*;
pub use introspect::introspect;
pub use json_schema::{GaranceSchema, load_schema, load_schema_from_str};
```

- [ ] **Step 3: Write tests**

```rust
// engine/crates/garance-engine/tests/json_schema_test.rs
use garance_engine::schema::json_schema::*;

#[test]
fn test_load_minimal_schema() {
    let json = r#"{
        "version": 1,
        "tables": {
            "users": {
                "columns": {
                    "id": { "type": "uuid", "primary_key": true, "default": "gen_random_uuid()" },
                    "email": { "type": "text", "unique": true, "nullable": false }
                }
            }
        }
    }"#;

    let schema = load_schema_from_str(json).unwrap();
    assert_eq!(schema.version, 1);
    assert_eq!(schema.tables.len(), 1);

    let users = schema.tables.get("users").unwrap();
    assert_eq!(users.columns.len(), 2);

    let id_col = users.columns.get("id").unwrap();
    assert!(id_col.primary_key);
    assert_eq!(id_col.col_type, "uuid");
    assert_eq!(id_col.default, Some("gen_random_uuid()".into()));
}

#[test]
fn test_load_schema_with_relations() {
    let json = r#"{
        "version": 1,
        "tables": {
            "users": {
                "columns": {
                    "id": { "type": "uuid", "primary_key": true }
                },
                "relations": {
                    "posts": { "type": "hasMany", "table": "posts", "foreign_key": "author_id" }
                }
            },
            "posts": {
                "columns": {
                    "id": { "type": "uuid", "primary_key": true },
                    "author_id": { "type": "uuid", "references": "users.id" }
                }
            }
        }
    }"#;

    let schema = load_schema_from_str(json).unwrap();
    let users = schema.tables.get("users").unwrap();
    let rel = users.relations.get("posts").unwrap();
    assert_eq!(rel.rel_type, "hasMany");
    assert_eq!(rel.table, "posts");
    assert_eq!(rel.foreign_key, "author_id");
}

#[test]
fn test_load_schema_with_access_rules() {
    let json = r#"{
        "version": 1,
        "tables": {
            "posts": {
                "columns": {
                    "id": { "type": "uuid", "primary_key": true },
                    "published": { "type": "bool" },
                    "author_id": { "type": "uuid" }
                },
                "access": {
                    "read": [
                        { "type": "where", "filters": { "published": true } },
                        { "type": "isOwner", "column": "author_id" }
                    ],
                    "write": [
                        { "type": "isOwner", "column": "author_id" }
                    ]
                }
            }
        }
    }"#;

    let schema = load_schema_from_str(json).unwrap();
    let posts = schema.tables.get("posts").unwrap();
    let access = posts.access.as_ref().unwrap();
    assert!(access.read.is_some());
    assert!(access.write.is_some());
    assert!(access.delete.is_none());
}

#[test]
fn test_load_schema_with_storage() {
    let json = r#"{
        "version": 1,
        "tables": {},
        "storage": {
            "avatars": {
                "max_file_size": "5mb",
                "allowed_mime_types": ["image/jpeg", "image/png"],
                "access": {
                    "read": "public",
                    "write": [{ "type": "isAuthenticated" }]
                }
            }
        }
    }"#;

    let schema = load_schema_from_str(json).unwrap();
    assert_eq!(schema.storage.len(), 1);

    let avatars = schema.storage.get("avatars").unwrap();
    assert_eq!(avatars.max_file_size, Some("5mb".into()));
    assert_eq!(avatars.allowed_mime_types, Some(vec!["image/jpeg".into(), "image/png".into()]));
}

#[test]
fn test_load_invalid_json_returns_error() {
    let result = load_schema_from_str("not json");
    assert!(result.is_err());
}
```

- [ ] **Step 4: Run tests**

Run: `cd engine && cargo test -p garance-engine`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add engine/
git commit -m ":sparkles: feat(engine): add garance.schema.json reader"
```

---

## Task 8: TypeScript Codegen

**Files:**
- Create: `engine/crates/garance-codegen/src/typescript.rs`
- Modify: `engine/crates/garance-codegen/src/lib.rs`
- Create: `engine/crates/garance-codegen/tests/typescript_test.rs`

- [ ] **Step 1: Write TypeScript type generator**

```rust
// engine/crates/garance-codegen/src/typescript.rs
use serde::Deserialize;
use std::collections::HashMap;
use std::fmt::Write;

#[derive(Debug, Deserialize)]
pub struct TableDef {
    pub name: String,
    pub columns: Vec<ColumnDef>,
}

#[derive(Debug, Deserialize)]
pub struct ColumnDef {
    pub name: String,
    pub data_type: String,
    pub is_nullable: bool,
    pub has_default: bool,
}

pub fn pg_type_to_ts(pg_type: &str) -> &str {
    match pg_type {
        "Uuid" | "Text" => "string",
        "Int4" | "Int8" | "Float8" | "Numeric" | "Serial" | "BigSerial" => "number",
        "Bool" => "boolean",
        "Timestamp" | "Timestamptz" | "Date" => "string",
        "Jsonb" | "Json" => "unknown",
        "Bytea" => "Uint8Array",
        _ => "unknown",
    }
}

pub fn generate_types(tables: &[TableDef]) -> String {
    let mut output = String::new();
    writeln!(output, "// Auto-generated by garance gen types --lang ts").unwrap();
    writeln!(output, "// Do not edit manually.\n").unwrap();

    for table in tables {
        let type_name = to_pascal_case(&table.name);

        // Row type (what you get back from queries)
        writeln!(output, "export interface {} {{", type_name).unwrap();
        for col in &table.columns {
            let ts_type = pg_type_to_ts(&col.data_type);
            let nullable = if col.is_nullable { " | null" } else { "" };
            writeln!(output, "  {}: {}{};", col.name, ts_type, nullable).unwrap();
        }
        writeln!(output, "}}\n").unwrap();

        // Insert type (omit columns with defaults, make nullable columns optional)
        writeln!(output, "export interface {}Insert {{", type_name).unwrap();
        for col in &table.columns {
            let ts_type = pg_type_to_ts(&col.data_type);
            let optional = if col.has_default || col.is_nullable { "?" } else { "" };
            let nullable = if col.is_nullable { " | null" } else { "" };
            writeln!(output, "  {}{}: {}{};", col.name, optional, ts_type, nullable).unwrap();
        }
        writeln!(output, "}}\n").unwrap();

        // Update type (all fields optional)
        writeln!(output, "export interface {}Update {{", type_name).unwrap();
        for col in &table.columns {
            let ts_type = pg_type_to_ts(&col.data_type);
            let nullable = if col.is_nullable { " | null" } else { "" };
            writeln!(output, "  {}?: {}{};", col.name, ts_type, nullable).unwrap();
        }
        writeln!(output, "}}\n").unwrap();
    }

    output
}

fn to_pascal_case(s: &str) -> String {
    s.split('_')
        .map(|word| {
            let mut chars = word.chars();
            match chars.next() {
                None => String::new(),
                Some(c) => c.to_uppercase().collect::<String>() + chars.as_str(),
            }
        })
        .collect()
}
```

- [ ] **Step 2: Update lib.rs**

```rust
// engine/crates/garance-codegen/src/lib.rs
pub mod typescript;

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}
```

- [ ] **Step 3: Write tests**

```rust
// engine/crates/garance-codegen/tests/typescript_test.rs
use garance_codegen::typescript::*;

#[test]
fn test_pg_type_mapping() {
    assert_eq!(pg_type_to_ts("Uuid"), "string");
    assert_eq!(pg_type_to_ts("Text"), "string");
    assert_eq!(pg_type_to_ts("Int4"), "number");
    assert_eq!(pg_type_to_ts("Bool"), "boolean");
    assert_eq!(pg_type_to_ts("Jsonb"), "unknown");
}

#[test]
fn test_generate_simple_types() {
    let tables = vec![
        TableDef {
            name: "users".into(),
            columns: vec![
                ColumnDef { name: "id".into(), data_type: "Uuid".into(), is_nullable: false, has_default: true },
                ColumnDef { name: "email".into(), data_type: "Text".into(), is_nullable: false, has_default: false },
                ColumnDef { name: "name".into(), data_type: "Text".into(), is_nullable: false, has_default: false },
                ColumnDef { name: "bio".into(), data_type: "Text".into(), is_nullable: true, has_default: false },
            ],
        }
    ];

    let output = generate_types(&tables);

    // Row type
    assert!(output.contains("export interface Users {"));
    assert!(output.contains("  id: string;"));
    assert!(output.contains("  email: string;"));
    assert!(output.contains("  bio: string | null;"));

    // Insert type — id has default so optional, bio is nullable so optional
    assert!(output.contains("export interface UsersInsert {"));
    assert!(output.contains("  id?: string;"));
    assert!(output.contains("  email: string;"));
    assert!(output.contains("  bio?: string | null;"));

    // Update type — all optional
    assert!(output.contains("export interface UsersUpdate {"));
    assert!(output.contains("  id?: string;"));
    assert!(output.contains("  email?: string;"));
}

#[test]
fn test_generate_multiple_tables() {
    let tables = vec![
        TableDef { name: "users".into(), columns: vec![] },
        TableDef { name: "blog_posts".into(), columns: vec![] },
    ];

    let output = generate_types(&tables);
    assert!(output.contains("export interface Users {"));
    assert!(output.contains("export interface BlogPosts {"));
}
```

- [ ] **Step 4: Run tests**

Run: `cd engine && cargo test -p garance-codegen`
Expected: All 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add engine/
git commit -m ":sparkles: feat(engine): add TypeScript type generation from PG schema"
```

---

## Task 9: Dockerfile

**Files:**
- Create: `engine/Dockerfile`

- [ ] **Step 1: Write multi-stage Dockerfile**

```dockerfile
# engine/Dockerfile
FROM rust:1.85-slim AS builder

WORKDIR /app
RUN apt-get update && apt-get install -y pkg-config libssl-dev && rm -rf /var/lib/apt/lists/*

# Cache dependencies
COPY Cargo.toml Cargo.lock ./
COPY crates/garance-pooler/Cargo.toml crates/garance-pooler/Cargo.toml
COPY crates/garance-engine/Cargo.toml crates/garance-engine/Cargo.toml
COPY crates/garance-codegen/Cargo.toml crates/garance-codegen/Cargo.toml
RUN mkdir -p crates/garance-pooler/src crates/garance-engine/src crates/garance-codegen/src \
    && echo "pub fn version() -> &'static str { \"0\" }" > crates/garance-pooler/src/lib.rs \
    && echo "pub fn version() -> &'static str { \"0\" }" > crates/garance-engine/src/lib.rs \
    && echo "fn main() {}" > crates/garance-engine/src/main.rs \
    && echo "pub fn version() -> &'static str { \"0\" }" > crates/garance-codegen/src/lib.rs \
    && cargo build --release && rm -rf crates/

# Build real source
COPY crates/ crates/
RUN touch crates/garance-engine/src/main.rs && cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/target/release/garance-engine /usr/local/bin/garance-engine

ENV LISTEN_ADDR=0.0.0.0:4000
EXPOSE 4000

CMD ["garance-engine"]
```

- [ ] **Step 2: Build and verify**

Run: `cd engine && docker build -t garance-engine:dev .`
Expected: Build succeeds, image created.

Run: `docker image ls garance-engine:dev`
Expected: Image listed, size < 100MB.

- [ ] **Step 3: Commit**

```bash
git add engine/Dockerfile
git commit -m ":whale: build(engine): add multi-stage Dockerfile"
```

---

## Summary

| Task | Description | Estimated Steps |
|---|---|---|
| 1 | Cargo workspace setup | 8 |
| 2 | Connection pooler with search_path | 7 |
| 3 | Core schema types | 6 |
| 4 | PG schema introspection | 5 |
| 5 | Query builder (filter → SQL) | 8 |
| 6 | HTTP REST API (axum) | 8 |
| 7 | Schema JSON reader | 5 |
| 8 | TypeScript codegen | 5 |
| 9 | Dockerfile | 3 |

**Total: 55 steps across 9 tasks**

### Not in this plan (deferred to later plans)

- gRPC interface (added when Gateway is built — Plan 4)
- Permission enforcement from access rules (depends on Auth service — Plan 2)
- Migration diff engine (garance.schema.json vs current PG schema → SQL migrations)
- OpenTelemetry tracing integration
- `POST /api/v1/rpc/{function}` — PG function calls (requires security review)
- `GET /api/v1/{table}/{id}/{relation}` — nested relation loading (requires join strategy design)

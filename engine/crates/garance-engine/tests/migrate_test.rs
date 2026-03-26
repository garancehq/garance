use std::sync::Arc;
use tokio::sync::RwLock;
use axum_test::TestServer;
use serde_json::json;
use testcontainers::runners::AsyncRunner;
use testcontainers_modules::postgres::Postgres;

use garance_engine::api;
use garance_engine::schema;
use garance_pooler::{GarancePool, PoolConfig};

async fn setup() -> (testcontainers::ContainerAsync<Postgres>, TestServer, Arc<GarancePool>) {
    let container = Postgres::default().start().await.unwrap();
    let port = container.get_host_port_ipv4(5432).await.unwrap();

    let config = PoolConfig { host: "127.0.0.1".into(), port, user: "postgres".into(), password: "postgres".into(), dbname: "postgres".into(), max_size: 4 };
    let pool = GarancePool::new(&config).unwrap();

    let client = pool.get().await.unwrap();
    schema::roles::ensure_roles(&client).await.unwrap();
    let db_schema = schema::introspect(&client, "public").await.unwrap();
    drop(client);

    let pool = Arc::new(pool);
    let state = api::state::AppState { pool: pool.clone(), schema: Arc::new(RwLock::new(db_schema)) };
    let app = api::router(state);
    let server = TestServer::new(app).unwrap();
    (container, server, pool)
}

#[tokio::test]
async fn test_migrate_preview_endpoint() {
    let (_container, server, _pool) = setup().await;

    let schema_json = json!({
        "schema": {
            "version": 1,
            "tables": {
                "articles": {
                    "columns": {
                        "id": { "type": "uuid", "primary_key": true, "default": "gen_random_uuid()" },
                        "title": { "type": "text", "nullable": false },
                        "body": { "type": "text", "nullable": true }
                    },
                    "relations": {}
                }
            },
            "storage": {}
        }
    });

    let response = server.post("/api/v1/_migrate/preview").json(&schema_json).await;
    response.assert_status_ok();

    let body: serde_json::Value = response.json();
    let statements = body["statements"].as_array().unwrap();
    assert!(!statements.is_empty(), "should produce at least one SQL statement");

    // Should contain a CREATE TABLE statement for articles
    let sql = statements.iter().map(|s| s.as_str().unwrap()).collect::<Vec<_>>().join("; ");
    assert!(sql.contains("articles"), "SQL should reference the articles table");
    assert!(sql.to_uppercase().contains("CREATE TABLE"), "SQL should contain CREATE TABLE");

    // Summary should show 1 table created
    assert_eq!(body["summary"]["tables_created"], 1);
    assert_eq!(body["has_destructive"], false);
}

#[tokio::test]
async fn test_migrate_apply_endpoint() {
    let (_container, server, pool) = setup().await;

    let sql = "CREATE TABLE articles (id SERIAL PRIMARY KEY, title TEXT NOT NULL)";
    let response = server.post("/api/v1/_migrate/apply").json(&json!({
        "sql": sql,
        "filename": "001_create_articles.sql"
    })).await;

    response.assert_status(axum::http::StatusCode::CREATED);

    let body: serde_json::Value = response.json();
    assert_eq!(body["applied"], true);
    assert_eq!(body["filename"], "001_create_articles.sql");
    assert!(body["tables_after"].as_u64().unwrap() >= 1, "should have at least 1 table after migration");

    // Verify the table actually exists
    let client = pool.get().await.unwrap();
    let rows = client.query(
        "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'articles'",
        &[],
    ).await.unwrap();
    assert_eq!(rows.len(), 1, "articles table should exist");

    // Verify tracking record
    let tracking = client.query(
        "SELECT filename, checksum FROM garance_platform.migrations WHERE filename = $1",
        &[&"001_create_articles.sql"],
    ).await.unwrap();
    assert_eq!(tracking.len(), 1, "tracking record should exist");
    let checksum: &str = tracking[0].get("checksum");
    assert!(!checksum.is_empty(), "checksum should not be empty");
}

#[tokio::test]
async fn test_migrate_tracking() {
    let (_container, server, _pool) = setup().await;

    let sql = "CREATE TABLE posts (id SERIAL PRIMARY KEY, content TEXT)";

    // Apply first time — should succeed
    let response = server.post("/api/v1/_migrate/apply").json(&json!({
        "sql": sql,
        "filename": "001_create_posts.sql"
    })).await;
    response.assert_status(axum::http::StatusCode::CREATED);

    // Apply same filename again — should return 409
    let response = server.post("/api/v1/_migrate/apply").json(&json!({
        "sql": sql,
        "filename": "001_create_posts.sql"
    })).await;
    response.assert_status(axum::http::StatusCode::CONFLICT);

    let body: serde_json::Value = response.json();
    assert_eq!(body["error"]["code"], "CONFLICT");
    assert!(body["error"]["message"].as_str().unwrap().contains("already applied"));
}

#[tokio::test]
async fn test_migrate_apply_rollback_on_error() {
    let (_container, server, pool) = setup().await;

    // Send invalid SQL
    let response = server.post("/api/v1/_migrate/apply").json(&json!({
        "sql": "CREATE TABLE bad_table (id SERIAL PRIMARY KEY); INVALID SQL HERE;",
        "filename": "002_bad_migration.sql"
    })).await;
    response.assert_status(axum::http::StatusCode::BAD_REQUEST);

    let body: serde_json::Value = response.json();
    assert_eq!(body["error"]["code"], "VALIDATION_ERROR");

    // Verify the table was NOT created (transaction rolled back)
    let client = pool.get().await.unwrap();
    let rows = client.query(
        "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'bad_table'",
        &[],
    ).await.unwrap();
    assert_eq!(rows.len(), 0, "bad_table should not exist after rollback");

    // Verify no tracking record was created
    // First ensure tracking table exists (might have been created by the handler)
    let tracking = client.query_opt(
        "SELECT filename FROM garance_platform.migrations WHERE filename = $1",
        &[&"002_bad_migration.sql"],
    ).await;
    match tracking {
        Ok(row) => assert!(row.is_none(), "no tracking record should exist after rollback"),
        Err(_) => {} // table doesn't exist at all, that's fine too
    }
}

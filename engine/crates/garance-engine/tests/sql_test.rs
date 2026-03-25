use std::sync::Arc;
use tokio::sync::RwLock;
use axum_test::TestServer;
use serde_json::json;
use testcontainers::runners::AsyncRunner;
use testcontainers_modules::postgres::Postgres;

use garance_engine::api;
use garance_engine::schema;
use garance_pooler::{GarancePool, PoolConfig};

async fn setup() -> (testcontainers::ContainerAsync<Postgres>, TestServer) {
    let container = Postgres::default().start().await.unwrap();
    let port = container.get_host_port_ipv4(5432).await.unwrap();

    let config = PoolConfig { host: "127.0.0.1".into(), port, user: "postgres".into(), password: "postgres".into(), dbname: "postgres".into(), max_size: 4 };
    let pool = GarancePool::new(&config).unwrap();

    let client = pool.get().await.unwrap();
    let _ = client.execute("CREATE EXTENSION IF NOT EXISTS pgcrypto", &[]).await;
    client.execute("CREATE TABLE todos (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), title text NOT NULL, done boolean DEFAULT false)", &[]).await.unwrap();
    client.execute("INSERT INTO todos (title) VALUES ('Task A'), ('Task B')", &[]).await.unwrap();
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
async fn test_sql_select() {
    let (_container, server) = setup().await;
    let response = server.post("/api/v1/rpc/query")
        .json(&json!({"sql": "SELECT title, done FROM todos ORDER BY title"}))
        .await;
    response.assert_status_ok();
    let body: serde_json::Value = response.json();
    assert_eq!(body["row_count"], 2);
    assert_eq!(body["columns"], json!(["title", "done"]));
    assert!(body["duration_ms"].is_number());
    assert_eq!(body["rows"][0]["title"], "Task A");
}

#[tokio::test]
async fn test_sql_insert_readonly() {
    let (_container, server) = setup().await;
    let response = server.post("/api/v1/rpc/query")
        .json(&json!({"sql": "INSERT INTO todos (title) VALUES ('new')"}))
        .await;
    // Should fail — read-only transaction
    response.assert_status(axum::http::StatusCode::BAD_REQUEST);
    let body: serde_json::Value = response.json();
    assert_eq!(body["error"]["code"], "VALIDATION_ERROR");
}

#[tokio::test]
async fn test_sql_insert_readwrite() {
    let (_container, server) = setup().await;
    let response = server.post("/api/v1/rpc/query")
        .add_header(axum::http::header::HeaderName::from_static("x-garance-sql-mode"), axum::http::header::HeaderValue::from_static("readwrite"))
        .json(&json!({"sql": "INSERT INTO todos (title) VALUES ('new') RETURNING *"}))
        .await;
    response.assert_status_ok();
    let body: serde_json::Value = response.json();
    assert_eq!(body["row_count"], 1);
    assert_eq!(body["rows"][0]["title"], "new");
}

#[tokio::test]
async fn test_sql_blocked_schema() {
    let (_container, server) = setup().await;
    let response = server.post("/api/v1/rpc/query")
        .json(&json!({"sql": "SELECT * FROM garance_auth.users"}))
        .await;
    response.assert_status(axum::http::StatusCode::BAD_REQUEST);
    let body: serde_json::Value = response.json();
    assert!(body["error"]["message"].as_str().unwrap().contains("garance_auth"));
}

#[tokio::test]
async fn test_sql_empty() {
    let (_container, server) = setup().await;
    let response = server.post("/api/v1/rpc/query")
        .json(&json!({"sql": ""}))
        .await;
    response.assert_status(axum::http::StatusCode::BAD_REQUEST);
    let body: serde_json::Value = response.json();
    assert!(body["error"]["message"].as_str().unwrap().contains("empty"));
}

#[tokio::test]
async fn test_sql_multi_statement() {
    let (_container, server) = setup().await;
    let response = server.post("/api/v1/rpc/query")
        .json(&json!({"sql": "SELECT 1; DROP TABLE todos"}))
        .await;
    response.assert_status(axum::http::StatusCode::BAD_REQUEST);
    let body: serde_json::Value = response.json();
    assert!(body["error"]["message"].as_str().unwrap().contains("multi-statement"));
}

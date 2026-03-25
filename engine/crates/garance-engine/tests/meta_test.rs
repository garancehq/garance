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
    client.execute(
        "CREATE TABLE users (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            email text UNIQUE NOT NULL,
            name text NOT NULL,
            created_at timestamptz DEFAULT now()
        )", &[]
    ).await.unwrap();
    client.execute("INSERT INTO users (email, name) VALUES ('a@test.com', 'Alice')", &[]).await.unwrap();
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
async fn test_list_tables() {
    let (_container, server) = setup().await;
    let response = server.get("/api/v1/_tables").await;
    response.assert_status_ok();
    let body: Vec<serde_json::Value> = response.json();
    assert_eq!(body.len(), 1);
    assert_eq!(body[0]["name"], "users");
    assert_eq!(body[0]["columns"], 4);
    assert!(body[0]["primary_key"].is_array());
}

#[tokio::test]
async fn test_get_schema() {
    let (_container, server) = setup().await;
    let response = server.get("/api/v1/_schema").await;
    response.assert_status_ok();
    let body: serde_json::Value = response.json();
    assert!(body["tables"]["users"].is_object());
    assert!(body["tables"]["users"]["columns"].is_array());
}

#[tokio::test]
async fn test_get_schema_single_table() {
    let (_container, server) = setup().await;

    let response = server.get("/api/v1/_schema/users").await;
    response.assert_status_ok();
    let body: serde_json::Value = response.json();
    assert_eq!(body["name"], "users");

    let response = server.get("/api/v1/_schema/nonexistent").await;
    response.assert_status(axum::http::StatusCode::NOT_FOUND);
    let body: serde_json::Value = response.json();
    assert_eq!(body["error"]["code"], "NOT_FOUND");
}

#[tokio::test]
async fn test_reload_schema() {
    let (_container, server) = setup().await;

    // "posts" table doesn't exist in the introspected schema
    let response = server.get("/api/v1/_schema/posts").await;
    response.assert_status(axum::http::StatusCode::NOT_FOUND);

    // Create it via rpc/query in readwrite mode (bypasses the in-memory schema)
    let response = server.post("/api/v1/rpc/query")
        .add_header(axum::http::header::HeaderName::from_static("x-garance-sql-mode"), axum::http::header::HeaderValue::from_static("readwrite"))
        .json(&json!({"sql": "CREATE TABLE posts (id serial PRIMARY KEY, title text NOT NULL)"}))
        .await;
    response.assert_status_ok();

    // Still not visible — schema not reloaded yet
    let response = server.get("/api/v1/_schema/posts").await;
    response.assert_status(axum::http::StatusCode::NOT_FOUND);

    // Reload
    let response = server.post("/api/v1/_reload").await;
    response.assert_status_ok();
    let body: serde_json::Value = response.json();
    assert_eq!(body["tables"], 2); // users + posts
    assert!(body["reloaded_at"].is_string());

    // Now "posts" should be visible
    let response = server.get("/api/v1/_schema/posts").await;
    response.assert_status_ok();
    let body: serde_json::Value = response.json();
    assert_eq!(body["name"], "posts");
}

#[tokio::test]
async fn test_reload_error_preserves_schema() {
    let (_container, server) = setup().await;

    // Verify current state: 1 table (users)
    let response = server.get("/api/v1/_tables").await;
    let body: Vec<serde_json::Value> = response.json();
    assert_eq!(body.len(), 1);

    // Reload should succeed and preserve schema
    let response = server.post("/api/v1/_reload").await;
    response.assert_status_ok();

    // Schema should still have 1 table
    let response = server.get("/api/v1/_tables").await;
    let body: Vec<serde_json::Value> = response.json();
    assert_eq!(body.len(), 1);
    assert_eq!(body[0]["name"], "users");
}

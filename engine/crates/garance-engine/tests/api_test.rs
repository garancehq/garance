use std::sync::Arc;
use tokio::sync::RwLock;
use axum_test::TestServer;
use serde_json::json;
use testcontainers::runners::AsyncRunner;
use testcontainers_modules::postgres::Postgres;

use garance_engine::api;
use garance_engine::schema;
use garance_pooler::{GarancePool, PoolConfig};

const USER_ID_HEADER: &str = "x-user-id";
const TEST_USER: &str = "test-user";

async fn setup() -> (testcontainers::ContainerAsync<Postgres>, TestServer) {
    let container = Postgres::default().start().await.unwrap();
    let port = container.get_host_port_ipv4(5432).await.unwrap();

    let config = PoolConfig { host: "127.0.0.1".into(), port, user: "postgres".into(), password: "postgres".into(), dbname: "postgres".into(), max_size: 4 };
    let pool = GarancePool::new(&config).unwrap();

    // Create test table — uuid PK. May need pgcrypto for gen_random_uuid() on older PG
    let client = pool.get().await.unwrap();
    let _ = client.execute("CREATE EXTENSION IF NOT EXISTS pgcrypto", &[]).await;

    // Ensure PG roles exist (required for SET LOCAL role in CRUD handlers)
    schema::roles::ensure_roles(&client).await.unwrap();

    client.execute(
        "CREATE TABLE users (
            id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
            name text NOT NULL,
            email text UNIQUE NOT NULL
        )", &[]
    ).await.unwrap();

    // Grant permissions to roles on the new table
    schema::roles::grant_table_permissions(&client, "users").await.unwrap();
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
    let response = server.post("/api/v1/users")
        .add_header(
            axum::http::HeaderName::from_static(USER_ID_HEADER),
            axum::http::HeaderValue::from_static(TEST_USER),
        )
        .json(&json!({"name": "Alice", "email": "alice@example.fr"}))
        .await;
    response.assert_status(axum::http::StatusCode::CREATED);
    let body: serde_json::Value = response.json();
    assert_eq!(body["name"], "Alice");
    assert_eq!(body["email"], "alice@example.fr");
    let id = body["id"].as_str().unwrap();
    let response = server.get(&format!("/api/v1/users/{}", id)).await;
    response.assert_status_ok();
    let body: serde_json::Value = response.json();
    assert_eq!(body["name"], "Alice");
}

#[tokio::test]
async fn test_update() {
    let (_container, server) = setup().await;
    let response = server.post("/api/v1/users")
        .add_header(
            axum::http::HeaderName::from_static(USER_ID_HEADER),
            axum::http::HeaderValue::from_static(TEST_USER),
        )
        .json(&json!({"name": "Bob", "email": "bob@example.fr"}))
        .await;
    let body: serde_json::Value = response.json();
    let id = body["id"].as_str().unwrap();
    let response = server.patch(&format!("/api/v1/users/{}", id))
        .add_header(
            axum::http::HeaderName::from_static(USER_ID_HEADER),
            axum::http::HeaderValue::from_static(TEST_USER),
        )
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
        .add_header(
            axum::http::HeaderName::from_static(USER_ID_HEADER),
            axum::http::HeaderValue::from_static(TEST_USER),
        )
        .json(&json!({"name": "Charlie", "email": "charlie@example.fr"}))
        .await;
    let body: serde_json::Value = response.json();
    let id = body["id"].as_str().unwrap();
    let response = server.delete(&format!("/api/v1/users/{}", id))
        .add_header(
            axum::http::HeaderName::from_static(USER_ID_HEADER),
            axum::http::HeaderValue::from_static(TEST_USER),
        )
        .await;
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
    server.post("/api/v1/users")
        .add_header(
            axum::http::HeaderName::from_static(USER_ID_HEADER),
            axum::http::HeaderValue::from_static(TEST_USER),
        )
        .json(&json!({"name": "Alice", "email": "a@x.fr"}))
        .await;
    server.post("/api/v1/users")
        .add_header(
            axum::http::HeaderName::from_static(USER_ID_HEADER),
            axum::http::HeaderValue::from_static(TEST_USER),
        )
        .json(&json!({"name": "Bob", "email": "b@x.fr"}))
        .await;
    let response = server.get("/api/v1/users?name=eq.Alice").await;
    response.assert_status_ok();
    let body: Vec<serde_json::Value> = response.json();
    assert_eq!(body.len(), 1);
    assert_eq!(body[0]["name"], "Alice");
}

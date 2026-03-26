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
        .json(&json!({"title": "Sneaky", "author_id": "user-2"}))
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
        .json(&json!({"title": "My Post", "author_id": "user-1"}))
        .await;
    response.assert_status(axum::http::StatusCode::CREATED);
}

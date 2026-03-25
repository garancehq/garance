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
    client.execute("CREATE EXTENSION IF NOT EXISTS pgcrypto", &[]).await.unwrap();
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

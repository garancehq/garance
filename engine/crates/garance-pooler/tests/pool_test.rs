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

    client.execute("CREATE SCHEMA project_abc123", &[]).await.unwrap();
    client.execute("CREATE TABLE project_abc123.users (id serial PRIMARY KEY, name text)", &[]).await.unwrap();

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
    assert_eq!(status.size, 0);
    assert_eq!(status.waiting, 0);
}

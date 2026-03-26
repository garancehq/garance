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

    let client = pool.get().await.expect("failed to connect to database");
    let db_schema = schema::introspect(&client, "public").await.expect("failed to introspect schema");

    // Setup realtime triggers
    schema::triggers::ensure_notify_function(&client).await.expect("failed to create notify function");
    schema::triggers::attach_triggers(&client, "public").await.expect("failed to attach triggers");

    // Ensure PG roles exist
    schema::roles::ensure_roles(&client).await.expect("failed to create PG roles");

    drop(client);

    info!(tables = db_schema.tables.len(), "schema introspected");

    let state = api::state::AppState {
        pool: Arc::new(pool),
        schema: Arc::new(RwLock::new(db_schema)),
    };

    let app = api::router(state.clone());

    // gRPC server
    let grpc_addr = std::env::var("GRPC_ADDR").unwrap_or_else(|_| "0.0.0.0:5000".into());
    let grpc_service = garance_engine::grpc::server::EngineGrpcService::new(
        state.pool.clone(),
        state.schema.clone(),
    );
    let grpc_listener = grpc_addr.parse().unwrap();
    tokio::spawn(async move {
        info!(%grpc_addr, "garance engine gRPC started");
        tonic::transport::Server::builder()
            .add_service(grpc_service.into_service())
            .serve(grpc_listener)
            .await
            .unwrap();
    });

    let addr = std::env::var("LISTEN_ADDR").unwrap_or_else(|_| "0.0.0.0:4000".into());
    let listener = tokio::net::TcpListener::bind(&addr).await.unwrap();
    info!(%addr, "garance engine started");
    axum::serve(listener, app).await.unwrap();
}

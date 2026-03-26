pub mod error;
pub mod routes;
pub mod sql_guard;
pub mod state;

use axum::{routing::get, routing::post, Router};
use state::AppState;

pub fn router(state: AppState) -> Router {
    Router::new()
        // CRUD
        .route("/api/v1/{table}", get(routes::list_rows).post(routes::insert_row))
        .route("/api/v1/{table}/{id}", get(routes::get_row).patch(routes::update_row).delete(routes::delete_row))
        // Meta endpoints (literal paths take priority over {table} in axum)
        .route("/api/v1/_tables", get(routes::list_tables))
        .route("/api/v1/_schema", get(routes::get_schema))
        .route("/api/v1/_schema/{table}", get(routes::get_schema_table))
        .route("/api/v1/_reload", post(routes::reload_schema))
        // Migrate endpoints
        .route("/api/v1/_migrate/preview", post(routes::migrate_preview))
        .route("/api/v1/_migrate/apply", post(routes::migrate_apply))
        // SQL execution
        .route("/api/v1/rpc/query", post(routes::execute_sql))
        // Health
        .route("/health", get(|| async { "ok" }))
        .with_state(state)
}

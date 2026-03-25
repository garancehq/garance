pub mod error;
pub mod routes;
pub mod state;

use axum::{routing::get, Router};
use state::AppState;

pub fn router(state: AppState) -> Router {
    Router::new()
        .route("/api/v1/{table}", get(routes::list_rows).post(routes::insert_row))
        .route("/api/v1/{table}/{id}", get(routes::get_row).patch(routes::update_row).delete(routes::delete_row))
        .route("/health", get(|| async { "ok" }))
        .with_state(state)
}

//! Garance Query Engine — introspection, query building, API serving.

pub mod schema;
pub mod query;
pub mod api;
pub mod grpc;
pub mod diff;

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}

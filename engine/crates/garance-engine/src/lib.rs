//! Garance Query Engine — introspection, query building, API serving.

pub mod schema;

pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}

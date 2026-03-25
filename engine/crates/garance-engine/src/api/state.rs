use std::sync::Arc;
use tokio::sync::RwLock;
use garance_pooler::GarancePool;
use crate::schema::types::Schema;

#[derive(Clone)]
pub struct AppState {
    pub pool: Arc<GarancePool>,
    pub schema: Arc<RwLock<Schema>>,
}

use deadpool_postgres::{Config, Pool, Runtime};
use tokio_postgres::NoTls;
use tracing::info;

use crate::config::PoolConfig;
use crate::error::{PoolError, Result};

pub struct GarancePool {
    inner: Pool,
}

impl GarancePool {
    pub fn new(config: &PoolConfig) -> Result<Self> {
        let mut cfg = Config::new();
        cfg.host = Some(config.host.clone());
        cfg.port = Some(config.port);
        cfg.user = Some(config.user.clone());
        cfg.password = Some(config.password.clone());
        cfg.dbname = Some(config.dbname.clone());
        cfg.pool = Some(deadpool_postgres::PoolConfig {
            max_size: config.max_size,
            ..Default::default()
        });

        let pool = cfg.create_pool(Some(Runtime::Tokio1), NoTls)
            .map_err(|e| PoolError::Config(e.to_string()))?;

        info!(host = %config.host, port = config.port, db = %config.dbname, max_size = config.max_size, "connection pool created");

        Ok(Self { inner: pool })
    }

    pub async fn get(&self) -> Result<deadpool_postgres::Client> {
        let client = self.inner.get().await?;
        Ok(client)
    }

    pub async fn get_for_project(&self, project_id: &str) -> Result<deadpool_postgres::Client> {
        let client = self.inner.get().await?;
        let schema = format!("project_{}", project_id);
        client.execute(
            &format!("SET search_path TO \"{}\", public", schema),
            &[],
        ).await?;
        Ok(client)
    }

    pub fn status(&self) -> PoolStatus {
        let status = self.inner.status();
        PoolStatus {
            size: status.size,
            available: status.available,
            waiting: status.waiting,
        }
    }
}

#[derive(Debug)]
pub struct PoolStatus {
    pub size: usize,
    pub available: usize,
    pub waiting: usize,
}

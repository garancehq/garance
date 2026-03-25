use thiserror::Error;

#[derive(Error, Debug)]
pub enum PoolError {
    #[error("failed to connect to PostgreSQL: {0}")]
    Connection(#[from] tokio_postgres::Error),

    #[error("pool exhausted: {0}")]
    Pool(#[from] deadpool_postgres::PoolError),

    #[error("invalid configuration: {0}")]
    Config(String),
}

pub type Result<T> = std::result::Result<T, PoolError>;

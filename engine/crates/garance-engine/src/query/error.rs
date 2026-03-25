use thiserror::Error;

#[derive(Error, Debug)]
pub enum QueryError {
    #[error("unknown table: {0}")]
    UnknownTable(String),
    #[error("unknown column '{column}' on table '{table}'")]
    UnknownColumn { table: String, column: String },
    #[error("invalid filter operator: {0}")]
    InvalidOperator(String),
    #[error("invalid filter value for column '{column}': {reason}")]
    InvalidValue { column: String, reason: String },
    #[error("database error: {0}")]
    Database(#[from] tokio_postgres::Error),
}

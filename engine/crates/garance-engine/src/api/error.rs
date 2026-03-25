use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::Json;
use serde::Serialize;
use crate::query::QueryError;

#[derive(Serialize)]
pub struct ApiError { pub error: ApiErrorBody }

#[derive(Serialize)]
pub struct ApiErrorBody {
    pub code: String, pub message: String, pub status: u16,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub details: Option<serde_json::Value>,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let status = StatusCode::from_u16(self.error.status).unwrap_or(StatusCode::INTERNAL_SERVER_ERROR);
        (status, Json(self)).into_response()
    }
}

impl From<QueryError> for ApiError {
    fn from(err: QueryError) -> Self {
        match &err {
            QueryError::UnknownTable(_) => ApiError { error: ApiErrorBody { code: "NOT_FOUND".into(), message: err.to_string(), status: 404, details: None } },
            QueryError::UnknownColumn { .. } | QueryError::InvalidOperator(_) | QueryError::InvalidValue { .. } => ApiError { error: ApiErrorBody { code: "VALIDATION_ERROR".into(), message: err.to_string(), status: 400, details: None } },
            QueryError::Database(_) => ApiError { error: ApiErrorBody { code: "INTERNAL_ERROR".into(), message: "internal database error".into(), status: 500, details: None } },
        }
    }
}

impl From<tokio_postgres::Error> for ApiError {
    fn from(_err: tokio_postgres::Error) -> Self {
        ApiError { error: ApiErrorBody { code: "INTERNAL_ERROR".into(), message: "internal database error".into(), status: 500, details: None } }
    }
}

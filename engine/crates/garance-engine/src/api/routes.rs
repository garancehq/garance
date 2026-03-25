use axum::extract::{Path, Query, State};
use axum::http::StatusCode;
use axum::response::IntoResponse;
use axum::Json;
use serde_json::{json, Value, Map};
use std::collections::HashMap;

use super::error::ApiError;
use super::state::AppState;
use crate::query::filter::parse_query_params;
use crate::query::builder::*;

pub async fn list_rows(
    State(state): State<AppState>,
    Path(table_name): Path<String>,
    Query(params): Query<HashMap<String, String>>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;
    let param_vec: Vec<(String, String)> = params.into_iter().collect();
    let qp = parse_query_params(&param_vec)?;
    let sql_query = build_select(table, &qp)?;
    let client = state.pool.get().await.map_err(|e| ApiError { error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None } })?;
    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> = sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();
    let rows = client.query(&sql_query.sql, &params_refs).await?;
    let results: Vec<Value> = rows.iter().map(row_to_json).collect();
    Ok(Json(results))
}

pub async fn get_row(
    State(state): State<AppState>,
    Path((table_name, id)): Path<(String, String)>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;
    let sql_query = build_select_by_id(table, &id)?;
    let client = state.pool.get().await.map_err(|e| ApiError { error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None } })?;
    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> = sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();
    let row = client.query_opt(&sql_query.sql, &params_refs).await?;
    match row {
        Some(row) => Ok(Json(row_to_json(&row))),
        None => Err(ApiError { error: super::error::ApiErrorBody { code: "NOT_FOUND".into(), message: format!("{} with id '{}' not found", table_name, id), status: 404, details: None } }),
    }
}

pub async fn insert_row(
    State(state): State<AppState>,
    Path(table_name): Path<String>,
    Json(body): Json<Map<String, Value>>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;
    let columns: Vec<String> = body.keys().cloned().collect();
    let values: Vec<String> = body.values().map(|v| match v { Value::String(s) => s.clone(), other => other.to_string() }).collect();
    let sql_query = build_insert(table, &columns)?;
    let client = state.pool.get().await.map_err(|e| ApiError { error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None } })?;
    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> = values.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();
    let row = client.query_one(&sql_query.sql, &params_refs).await?;
    Ok((StatusCode::CREATED, Json(row_to_json(&row))))
}

pub async fn update_row(
    State(state): State<AppState>,
    Path((table_name, id)): Path<(String, String)>,
    Json(body): Json<Map<String, Value>>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;
    let columns: Vec<String> = body.keys().cloned().collect();
    let values: Vec<String> = body.values().map(|v| match v { Value::String(s) => s.clone(), other => other.to_string() }).collect();
    let sql_query = build_update(table, &id, &columns)?;
    let client = state.pool.get().await.map_err(|e| ApiError { error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None } })?;
    let mut all_params: Vec<String> = values;
    all_params.push(id.clone());
    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> = all_params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();
    let row = client.query_opt(&sql_query.sql, &params_refs).await?;
    match row {
        Some(row) => Ok(Json(row_to_json(&row))),
        None => Err(ApiError { error: super::error::ApiErrorBody { code: "NOT_FOUND".into(), message: format!("{} with id '{}' not found", table_name, id), status: 404, details: None } }),
    }
}

pub async fn delete_row(
    State(state): State<AppState>,
    Path((table_name, id)): Path<(String, String)>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    let table = schema.tables.get(&table_name)
        .ok_or_else(|| ApiError::from(crate::query::QueryError::UnknownTable(table_name.clone())))?;
    let sql_query = build_delete(table, &id)?;
    let client = state.pool.get().await.map_err(|e| ApiError { error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None } })?;
    let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> = sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();
    let affected = client.execute(&sql_query.sql, &params_refs).await?;
    if affected == 0 { return Err(ApiError { error: super::error::ApiErrorBody { code: "NOT_FOUND".into(), message: format!("{} with id '{}' not found", table_name, id), status: 404, details: None } }); }
    Ok(StatusCode::NO_CONTENT)
}

pub fn row_to_json(row: &tokio_postgres::Row) -> Value {
    use tokio_postgres::types::Type;
    let mut obj = Map::new();
    for (i, col) in row.columns().iter().enumerate() {
        let value = match *col.type_() {
            Type::BOOL => row.get::<_, Option<bool>>(i).map(Value::Bool).unwrap_or(Value::Null),
            Type::INT2 => row.get::<_, Option<i16>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::INT4 => row.get::<_, Option<i32>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::INT8 => row.get::<_, Option<i64>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::FLOAT4 => row.get::<_, Option<f32>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::FLOAT8 => row.get::<_, Option<f64>>(i).map(|v| json!(v)).unwrap_or(Value::Null),
            Type::UUID => row.get::<_, Option<uuid::Uuid>>(i).map(|v| Value::String(v.to_string())).unwrap_or(Value::Null),
            Type::JSON | Type::JSONB => row.get::<_, Option<serde_json::Value>>(i).unwrap_or(Value::Null),
            _ => { row.get::<_, Option<&str>>(i).map(|v| Value::String(v.to_string())).unwrap_or(Value::Null) }
        };
        obj.insert(col.name().to_string(), value);
    }
    Value::Object(obj)
}

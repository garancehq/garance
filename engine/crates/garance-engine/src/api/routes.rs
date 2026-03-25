use axum::extract::{Path, Query, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::IntoResponse;
use axum::Json;
use serde::Deserialize;
use serde_json::{json, Value, Map};
use std::collections::HashMap;

use super::error::ApiError;
use super::sql_guard;
use super::state::AppState;
use crate::query::filter::parse_query_params;
use crate::query::builder::*;

// ─── Meta endpoints ──────────────────────────────────────────────────────────

/// GET /api/v1/_tables — list introspected tables with metadata
pub async fn list_tables(
    State(state): State<AppState>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;

    // Get approximate row counts from pg_stat_user_tables
    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
    })?;

    let stat_rows = client.query(
        "SELECT relname, n_live_tup FROM pg_stat_user_tables WHERE schemaname = 'public'",
        &[],
    ).await.unwrap_or_default();

    let mut row_counts: HashMap<String, i64> = HashMap::new();
    for row in &stat_rows {
        let name: &str = row.get("relname");
        let count: i64 = row.get("n_live_tup");
        row_counts.insert(name.to_string(), count);
    }

    const RESERVED_NAMES: &[&str] = &["_tables", "_schema", "_reload", "rpc"];

    let mut tables: Vec<Value> = schema.tables.iter()
        .filter(|(name, _)| !RESERVED_NAMES.contains(&name.as_str()))
        .map(|(name, table)| {
        json!({
            "name": name,
            "columns": table.columns.len(),
            "primary_key": table.primary_key,
            "row_count": row_counts.get(name).copied(),
        })
    }).collect();
    tables.sort_by(|a, b| a["name"].as_str().cmp(&b["name"].as_str()));

    Ok(Json(tables))
}

/// GET /api/v1/_schema — full introspected schema
pub async fn get_schema(
    State(state): State<AppState>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    Ok(Json(json!(*schema)))
}

/// GET /api/v1/_schema/{table} — schema for a single table
pub async fn get_schema_table(
    State(state): State<AppState>,
    Path(table_name): Path<String>,
) -> Result<impl IntoResponse, ApiError> {
    let schema = state.schema.read().await;
    match schema.tables.get(&table_name) {
        Some(table) => Ok(Json(json!(table))),
        None => Err(ApiError {
            error: super::error::ApiErrorBody {
                code: "NOT_FOUND".into(),
                message: format!("table '{}' not found", table_name),
                status: 404,
                details: None,
            },
        }),
    }
}

/// POST /api/v1/_reload — re-introspect PG schema without restart
pub async fn reload_schema(
    State(state): State<AppState>,
) -> Result<impl IntoResponse, ApiError> {
    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
    })?;

    // Introspect FIRST, before acquiring write lock — error safety
    let new_schema = crate::schema::introspect(&client, "public").await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: format!("introspection failed: {}", e), status: 500, details: None },
    })?;

    let table_count = new_schema.tables.len();
    let mut schema = state.schema.write().await;
    *schema = new_schema;

    Ok(Json(json!({
        "tables": table_count,
        "reloaded_at": chrono::Utc::now().to_rfc3339(),
    })))
}

// ─── SQL execution ───────────────────────────────────────────────────────────

#[derive(Deserialize)]
pub struct SqlRequest {
    sql: String,
}

/// POST /api/v1/rpc/query — execute scoped SQL
pub async fn execute_sql(
    State(state): State<AppState>,
    headers: HeaderMap,
    Json(body): Json<SqlRequest>,
) -> Result<impl IntoResponse, ApiError> {
    // Validate SQL
    sql_guard::validate_sql(&body.sql).map_err(|e| ApiError {
        error: super::error::ApiErrorBody {
            code: "VALIDATION_ERROR".into(),
            message: e.to_string(),
            status: 400,
            details: None,
        },
    })?;

    // Check readwrite mode
    let readwrite = headers
        .get("x-garance-sql-mode")
        .and_then(|v| v.to_str().ok())
        .map(|v| v.eq_ignore_ascii_case("readwrite"))
        .unwrap_or(false);

    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
    })?;

    let start = std::time::Instant::now();

    // Helper to map PG errors in transaction setup
    let pg_err = |e: tokio_postgres::Error| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
    };

    // Begin transaction with scoped search_path
    client.execute("BEGIN", &[]).await.map_err(&pg_err)?;
    client.execute("SET LOCAL search_path TO public", &[]).await.map_err(&pg_err)?;
    if !readwrite {
        client.execute("SET TRANSACTION READ ONLY", &[]).await.map_err(&pg_err)?;
    }

    // Execute and handle result
    let result = client.query(&body.sql, &[]).await;

    match result {
        Ok(rows) => {
            client.execute("COMMIT", &[]).await.map_err(&pg_err)?;
            let duration_ms = start.elapsed().as_millis() as u64;

            let columns: Vec<String> = if rows.is_empty() {
                vec![]
            } else {
                rows[0].columns().iter().map(|c| c.name().to_string()).collect()
            };

            let json_rows: Vec<Value> = rows.iter().map(row_to_json).collect();
            let row_count = json_rows.len();

            Ok(Json(json!({
                "columns": columns,
                "rows": json_rows,
                "row_count": row_count,
                "duration_ms": duration_ms,
            })))
        }
        Err(e) => {
            let _ = client.execute("ROLLBACK", &[]).await;
            let duration_ms = start.elapsed().as_millis() as u64;

            Err(ApiError {
                error: super::error::ApiErrorBody {
                    code: "VALIDATION_ERROR".into(),
                    message: format!("SQL error: {}", e),
                    status: 400,
                    details: Some(json!({ "duration_ms": duration_ms })),
                },
            })
        }
    }
}

// ─── CRUD endpoints ──────────────────────────────────────────────────────────

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
            Type::TIMESTAMP => row.get::<_, Option<chrono::NaiveDateTime>>(i).map(|v| Value::String(v.format("%Y-%m-%dT%H:%M:%S").to_string())).unwrap_or(Value::Null),
            Type::TIMESTAMPTZ => row.get::<_, Option<chrono::DateTime<chrono::Utc>>>(i).map(|v| Value::String(v.to_rfc3339())).unwrap_or(Value::Null),
            Type::DATE => row.get::<_, Option<chrono::NaiveDate>>(i).map(|v| Value::String(v.to_string())).unwrap_or(Value::Null),
            Type::TEXT | Type::VARCHAR | Type::BPCHAR | Type::NAME => row.get::<_, Option<&str>>(i).map(|v| Value::String(v.to_string())).unwrap_or(Value::Null),
            _ => {
                // Last resort: try as string, if that fails return type name
                match row.try_get::<_, Option<&str>>(i) {
                    Ok(v) => v.map(|s| Value::String(s.to_string())).unwrap_or(Value::Null),
                    Err(_) => Value::String(format!("<{}>", col.type_().name())),
                }
            }
        };
        obj.insert(col.name().to_string(), value);
    }
    Value::Object(obj)
}

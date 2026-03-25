use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use tonic::{Request, Response, Status};
use serde_json::{Map, Value};

use crate::api::routes::row_to_json;
use crate::api::sql_guard;
use crate::query::filter::parse_query_params;
use crate::query::builder::*;
use crate::schema::types::Schema;
use garance_pooler::GarancePool;

pub mod engine_proto {
    tonic::include_proto!("engine.v1");
}

use engine_proto::engine_service_server::{EngineService, EngineServiceServer};
use engine_proto::*;

pub struct EngineGrpcService {
    pool: Arc<GarancePool>,
    schema: Arc<RwLock<Schema>>,
}

impl EngineGrpcService {
    pub fn new(pool: Arc<GarancePool>, schema: Arc<RwLock<Schema>>) -> Self {
        Self { pool, schema }
    }

    pub fn into_service(self) -> EngineServiceServer<Self> {
        EngineServiceServer::new(self)
    }
}

#[tonic::async_trait]
impl EngineService for EngineGrpcService {
    async fn list_rows(&self, request: Request<ListRowsRequest>) -> Result<Response<ListRowsResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let mut params: Vec<(String, String)> = req.filters.into_iter().collect();
        if !req.select.is_empty() {
            params.push(("select".into(), req.select));
        }
        if !req.order.is_empty() {
            params.push(("order".into(), req.order));
        }
        if req.limit > 0 {
            params.push(("limit".into(), req.limit.to_string()));
        }
        if req.offset > 0 {
            params.push(("offset".into(), req.offset.to_string()));
        }

        let qp = parse_query_params(&params).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let sql_query = build_select(table, &qp).map_err(|e| Status::invalid_argument(e.to_string()))?;

        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let rows = client.query(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        let results: Vec<Value> = rows.iter().map(row_to_json).collect();
        let json_bytes = serde_json::to_vec(&results).map_err(|e| Status::internal(e.to_string()))?;

        Ok(Response::new(ListRowsResponse {
            rows_json: json_bytes,
            count: results.len() as i64,
        }))
    }

    async fn get_row(&self, request: Request<GetRowRequest>) -> Result<Response<GetRowResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let sql_query = build_select_by_id(table, &req.id).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let row = client.query_opt(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        match row {
            Some(row) => {
                let json_bytes = serde_json::to_vec(&row_to_json(&row)).map_err(|e| Status::internal(e.to_string()))?;
                Ok(Response::new(GetRowResponse { row_json: json_bytes, found: true }))
            }
            None => Ok(Response::new(GetRowResponse { row_json: vec![], found: false })),
        }
    }

    async fn insert_row(&self, request: Request<InsertRowRequest>) -> Result<Response<InsertRowResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let body: Map<String, Value> = serde_json::from_slice(&req.body_json)
            .map_err(|e| Status::invalid_argument(format!("invalid JSON body: {}", e)))?;

        let columns: Vec<String> = body.keys().cloned().collect();
        let values: Vec<String> = body.values().map(|v| match v {
            Value::String(s) => s.clone(),
            other => other.to_string(),
        }).collect();

        let sql_query = build_insert(table, &columns).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            values.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let row = client.query_one(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        let json_bytes = serde_json::to_vec(&row_to_json(&row)).map_err(|e| Status::internal(e.to_string()))?;
        Ok(Response::new(InsertRowResponse { row_json: json_bytes }))
    }

    async fn update_row(&self, request: Request<UpdateRowRequest>) -> Result<Response<UpdateRowResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let body: Map<String, Value> = serde_json::from_slice(&req.body_json)
            .map_err(|e| Status::invalid_argument(format!("invalid JSON body: {}", e)))?;

        let columns: Vec<String> = body.keys().cloned().collect();
        let values: Vec<String> = body.values().map(|v| match v {
            Value::String(s) => s.clone(),
            other => other.to_string(),
        }).collect();

        let sql_query = build_update(table, &req.id, &columns).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;

        let mut all_params: Vec<String> = values;
        all_params.push(req.id);
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            all_params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let row = client.query_opt(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        match row {
            Some(row) => {
                let json_bytes = serde_json::to_vec(&row_to_json(&row)).map_err(|e| Status::internal(e.to_string()))?;
                Ok(Response::new(UpdateRowResponse { row_json: json_bytes, found: true }))
            }
            None => Ok(Response::new(UpdateRowResponse { row_json: vec![], found: false })),
        }
    }

    async fn delete_row(&self, request: Request<DeleteRowRequest>) -> Result<Response<DeleteRowResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;
        let table = schema.tables.get(&req.table)
            .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;

        let sql_query = build_delete(table, &req.id).map_err(|e| Status::invalid_argument(e.to_string()))?;
        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
        let params_refs: Vec<&(dyn tokio_postgres::types::ToSql + Sync)> =
            sql_query.params.iter().map(|s| s as &(dyn tokio_postgres::types::ToSql + Sync)).collect();

        let affected = client.execute(&sql_query.sql, &params_refs).await
            .map_err(|e| Status::internal(e.to_string()))?;

        Ok(Response::new(DeleteRowResponse { found: affected > 0 }))
    }

    // ─── Meta RPCs ──────────────────────────────────────────────────────────

    async fn list_tables(&self, _request: Request<ListTablesRequest>) -> Result<Response<ListTablesResponse>, Status> {
        let schema = self.schema.read().await;

        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
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

        let mut tables: Vec<TableSummary> = schema.tables.iter()
            .filter(|(name, _)| !RESERVED_NAMES.contains(&name.as_str()))
            .map(|(name, table)| {
                TableSummary {
                    name: name.clone(),
                    columns: table.columns.len() as i32,
                    primary_key: table.primary_key.clone().unwrap_or_default(),
                    row_count: row_counts.get(name).copied().unwrap_or(0),
                }
            }).collect();
        tables.sort_by(|a, b| a.name.cmp(&b.name));

        Ok(Response::new(ListTablesResponse { tables }))
    }

    async fn get_schema(&self, request: Request<GetSchemaRequest>) -> Result<Response<GetSchemaResponse>, Status> {
        let req = request.into_inner();
        let schema = self.schema.read().await;

        let schema_json = if req.table.is_empty() {
            serde_json::to_vec(&*schema).map_err(|e| Status::internal(e.to_string()))?
        } else {
            let table = schema.tables.get(&req.table)
                .ok_or_else(|| Status::not_found(format!("table '{}' not found", req.table)))?;
            serde_json::to_vec(table).map_err(|e| Status::internal(e.to_string()))?
        };

        Ok(Response::new(GetSchemaResponse { schema_json }))
    }

    async fn execute_sql(&self, request: Request<ExecuteSqlRequest>) -> Result<Response<ExecuteSqlResponse>, Status> {
        let req = request.into_inner();

        sql_guard::validate_sql(&req.sql)
            .map_err(|e| Status::invalid_argument(e.to_string()))?;

        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;
        let start = std::time::Instant::now();

        // Begin transaction with scoped search_path
        client.execute("BEGIN", &[]).await.map_err(|e| Status::internal(e.to_string()))?;
        client.execute("SET LOCAL search_path TO public", &[]).await.map_err(|e| Status::internal(e.to_string()))?;
        if !req.readwrite {
            client.execute("SET TRANSACTION READ ONLY", &[]).await.map_err(|e| Status::internal(e.to_string()))?;
        }

        let result = client.query(&req.sql, &[]).await;

        match result {
            Ok(rows) => {
                client.execute("COMMIT", &[]).await.map_err(|e| Status::internal(e.to_string()))?;
                let duration_ms = start.elapsed().as_millis() as i64;

                let columns: Vec<String> = if rows.is_empty() {
                    vec![]
                } else {
                    rows[0].columns().iter().map(|c| c.name().to_string()).collect()
                };

                let json_rows: Vec<Value> = rows.iter().map(row_to_json).collect();
                let row_count = json_rows.len() as i64;
                let rows_json = serde_json::to_vec(&json_rows).map_err(|e| Status::internal(e.to_string()))?;

                Ok(Response::new(ExecuteSqlResponse {
                    columns,
                    rows_json,
                    row_count,
                    duration_ms,
                }))
            }
            Err(e) => {
                let _ = client.execute("ROLLBACK", &[]).await;
                Err(Status::invalid_argument(format!("SQL error: {}", e)))
            }
        }
    }

    async fn reload_schema(&self, _request: Request<ReloadSchemaRequest>) -> Result<Response<ReloadSchemaResponse>, Status> {
        let client = self.pool.get().await.map_err(|e| Status::internal(e.to_string()))?;

        // Introspect FIRST, before acquiring write lock — error safety
        let new_schema = crate::schema::introspect(&client, "public").await
            .map_err(|e| Status::internal(format!("introspection failed: {}", e)))?;

        let table_count = new_schema.tables.len() as i32;
        let mut schema = self.schema.write().await;
        *schema = new_schema;

        Ok(Response::new(ReloadSchemaResponse {
            tables: table_count,
            reloaded_at: chrono::Utc::now().to_rfc3339(),
        }))
    }
}

use std::sync::Arc;
use tokio::sync::RwLock;
use tonic::{Request, Response, Status};
use serde_json::{Map, Value};

use crate::api::routes::row_to_json;
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
}

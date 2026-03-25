use crate::schema::types::Table;
use super::filter::{QueryParams, SortDirection, Operator};
use super::error::QueryError;

#[derive(Debug)]
pub struct SqlQuery { pub sql: String, pub params: Vec<String> }

pub fn build_select(table: &Table, qp: &QueryParams) -> Result<SqlQuery, QueryError> {
    let mut params: Vec<String> = vec![];
    let mut param_idx = 1;
    let columns = match &qp.select {
        Some(cols) => {
            for col in cols { if table.column(col).is_none() { return Err(QueryError::UnknownColumn { table: table.name.clone(), column: col.clone() }); } }
            cols.iter().map(|c| format!("\"{}\"", c)).collect::<Vec<_>>().join(", ")
        }
        None => "*".to_string(),
    };
    let mut sql = format!("SELECT {} FROM \"{}\"", columns, table.name);
    if !qp.filters.is_empty() {
        let mut conditions = vec![];
        for filter in &qp.filters {
            if table.column(&filter.column).is_none() { return Err(QueryError::UnknownColumn { table: table.name.clone(), column: filter.column.clone() }); }
            match filter.operator {
                Operator::Is => {
                    let val = match filter.value.to_lowercase().as_str() { "null" => "NULL", "true" => "TRUE", "false" => "FALSE", _ => return Err(QueryError::InvalidValue { column: filter.column.clone(), reason: "IS only supports null, true, false".into() }) };
                    conditions.push(format!("\"{}\" IS {}", filter.column, val));
                }
                Operator::In => {
                    let values: Vec<&str> = filter.value.split(',').collect();
                    let placeholders: Vec<String> = values.iter().map(|v| { params.push(v.trim().to_string()); let p = format!("${}", param_idx); param_idx += 1; p }).collect();
                    conditions.push(format!("\"{}\"::text IN ({})", filter.column, placeholders.join(", ")));
                }
                _ => { params.push(filter.value.clone()); conditions.push(format!("\"{}\"::text {} ${}", filter.column, filter.operator.to_sql(), param_idx)); param_idx += 1; }
            }
        }
        sql.push_str(&format!(" WHERE {}", conditions.join(" AND ")));
    }
    if !qp.order.is_empty() {
        let order_parts: Vec<String> = qp.order.iter().map(|s| { let dir = match s.direction { SortDirection::Asc => "ASC", SortDirection::Desc => "DESC" }; format!("\"{}\" {}", s.column, dir) }).collect();
        sql.push_str(&format!(" ORDER BY {}", order_parts.join(", ")));
    }
    if let Some(limit) = qp.limit { sql.push_str(&format!(" LIMIT {}", limit)); }
    if let Some(offset) = qp.offset { sql.push_str(&format!(" OFFSET {}", offset)); }
    Ok(SqlQuery { sql, params })
}

pub fn build_select_by_id(table: &Table, id_value: &str) -> Result<SqlQuery, QueryError> {
    let pk = table.primary_key.as_ref().ok_or_else(|| QueryError::UnknownColumn { table: table.name.clone(), column: "(primary key)".into() })?;
    let pk_col = &pk[0];
    let sql = format!("SELECT * FROM \"{}\" WHERE \"{}\"::text = $1", table.name, pk_col);
    Ok(SqlQuery { sql, params: vec![id_value.to_string()] })
}

pub fn build_insert(table: &Table, columns: &[String]) -> Result<SqlQuery, QueryError> {
    for col in columns { if table.column(col).is_none() { return Err(QueryError::UnknownColumn { table: table.name.clone(), column: col.clone() }); } }
    let col_names: Vec<String> = columns.iter().map(|c| format!("\"{}\"", c)).collect();
    let placeholders: Vec<String> = (1..=columns.len()).map(|i| format!("${}", i)).collect();
    let sql = format!("INSERT INTO \"{}\" ({}) VALUES ({}) RETURNING *", table.name, col_names.join(", "), placeholders.join(", "));
    Ok(SqlQuery { sql, params: vec![] })
}

pub fn build_update(table: &Table, id_value: &str, columns: &[String]) -> Result<SqlQuery, QueryError> {
    let pk = table.primary_key.as_ref().ok_or_else(|| QueryError::UnknownColumn { table: table.name.clone(), column: "(primary key)".into() })?;
    for col in columns { if table.column(col).is_none() { return Err(QueryError::UnknownColumn { table: table.name.clone(), column: col.clone() }); } }
    let set_clauses: Vec<String> = columns.iter().enumerate().map(|(i, col)| format!("\"{}\" = ${}", col, i + 1)).collect();
    let pk_col = &pk[0];
    let sql = format!("UPDATE \"{}\" SET {} WHERE \"{}\"::text = ${} RETURNING *", table.name, set_clauses.join(", "), pk_col, columns.len() + 1);
    Ok(SqlQuery { sql, params: vec![id_value.to_string()] })
}

pub fn build_delete(table: &Table, id_value: &str) -> Result<SqlQuery, QueryError> {
    let pk = table.primary_key.as_ref().ok_or_else(|| QueryError::UnknownColumn { table: table.name.clone(), column: "(primary key)".into() })?;
    let pk_col = &pk[0];
    let sql = format!("DELETE FROM \"{}\" WHERE \"{}\"::text = $1", table.name, pk_col);
    Ok(SqlQuery { sql, params: vec![id_value.to_string()] })
}

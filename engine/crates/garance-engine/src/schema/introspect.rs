use tokio_postgres::Client;
use std::collections::HashMap;
use tracing::info;

use super::types::*;

pub async fn introspect(client: &Client, schema_name: &str) -> Result<Schema, tokio_postgres::Error> {
    let tables = introspect_tables(client, schema_name).await?;
    info!(schema = schema_name, table_count = tables.len(), "schema introspected");
    Ok(Schema { tables })
}

async fn introspect_tables(client: &Client, schema_name: &str) -> Result<HashMap<String, Table>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT table_name FROM information_schema.tables
         WHERE table_schema = $1 AND table_type = 'BASE TABLE'
         ORDER BY table_name",
        &[&schema_name],
    ).await?;

    let mut tables = HashMap::new();
    for row in &rows {
        let table_name: &str = row.get("table_name");
        let columns = introspect_columns(client, schema_name, table_name).await?;
        let primary_key = introspect_primary_key(client, schema_name, table_name).await?;
        let foreign_keys = introspect_foreign_keys(client, schema_name, table_name).await?;
        let indexes = introspect_indexes(client, schema_name, table_name).await?;

        tables.insert(table_name.to_string(), Table {
            name: table_name.to_string(),
            schema: schema_name.to_string(),
            columns,
            primary_key,
            foreign_keys,
            indexes,
        });
    }

    Ok(tables)
}

async fn introspect_columns(client: &Client, schema_name: &str, table_name: &str) -> Result<Vec<Column>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT
            c.column_name,
            c.udt_name,
            c.is_nullable,
            c.column_default,
            EXISTS (
                SELECT 1 FROM information_schema.table_constraints tc
                JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
                WHERE tc.table_schema = $1 AND tc.table_name = $2
                AND kcu.column_name = c.column_name AND tc.constraint_type = 'PRIMARY KEY'
            ) as is_pk,
            EXISTS (
                SELECT 1 FROM information_schema.table_constraints tc
                JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
                WHERE tc.table_schema = $1 AND tc.table_name = $2
                AND kcu.column_name = c.column_name AND tc.constraint_type = 'UNIQUE'
            ) as is_unique
         FROM information_schema.columns c
         WHERE c.table_schema = $1 AND c.table_name = $2
         ORDER BY c.ordinal_position",
        &[&schema_name, &table_name],
    ).await?;

    Ok(rows.iter().map(|row| {
        let udt_name: &str = row.get("udt_name");
        let is_nullable: &str = row.get("is_nullable");
        let default_value: Option<&str> = row.get("column_default");

        Column {
            name: row.get::<_, &str>("column_name").to_string(),
            data_type: PgType::from_pg(udt_name),
            is_nullable: is_nullable == "YES",
            has_default: default_value.is_some(),
            default_value: default_value.map(String::from),
            is_primary_key: row.get("is_pk"),
            is_unique: row.get("is_unique"),
        }
    }).collect())
}

async fn introspect_primary_key(client: &Client, schema_name: &str, table_name: &str) -> Result<Option<Vec<String>>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT kcu.column_name
         FROM information_schema.table_constraints tc
         JOIN information_schema.key_column_usage kcu
           ON tc.constraint_name = kcu.constraint_name
           AND tc.table_schema = kcu.table_schema
         WHERE tc.table_schema = $1
           AND tc.table_name = $2
           AND tc.constraint_type = 'PRIMARY KEY'
         ORDER BY kcu.ordinal_position",
        &[&schema_name, &table_name],
    ).await?;

    if rows.is_empty() {
        return Ok(None);
    }

    Ok(Some(rows.iter().map(|r| r.get::<_, &str>("column_name").to_string()).collect()))
}

async fn introspect_foreign_keys(client: &Client, schema_name: &str, table_name: &str) -> Result<Vec<ForeignKey>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT
            tc.constraint_name,
            kcu.column_name,
            ccu.table_name AS referenced_table,
            ccu.column_name AS referenced_column
         FROM information_schema.table_constraints tc
         JOIN information_schema.key_column_usage kcu
           ON tc.constraint_name = kcu.constraint_name
           AND tc.table_schema = kcu.table_schema
         JOIN information_schema.constraint_column_usage ccu
           ON tc.constraint_name = ccu.constraint_name
           AND tc.table_schema = ccu.table_schema
         WHERE tc.table_schema = $1
           AND tc.table_name = $2
           AND tc.constraint_type = 'FOREIGN KEY'
         ORDER BY tc.constraint_name, kcu.ordinal_position",
        &[&schema_name, &table_name],
    ).await?;

    let mut fks_by_constraint: HashMap<String, ForeignKey> = HashMap::new();
    for row in &rows {
        let constraint: String = row.get::<_, &str>("constraint_name").to_string();
        let col: String = row.get::<_, &str>("column_name").to_string();
        let ref_table: String = row.get::<_, &str>("referenced_table").to_string();
        let ref_col: String = row.get::<_, &str>("referenced_column").to_string();

        let fk = fks_by_constraint.entry(constraint).or_insert_with(|| ForeignKey {
            columns: vec![],
            referenced_table: ref_table,
            referenced_columns: vec![],
        });
        if !fk.columns.contains(&col) {
            fk.columns.push(col);
        }
        if !fk.referenced_columns.contains(&ref_col) {
            fk.referenced_columns.push(ref_col);
        }
    }

    Ok(fks_by_constraint.into_values().collect())
}

async fn introspect_indexes(client: &Client, schema_name: &str, table_name: &str) -> Result<Vec<Index>, tokio_postgres::Error> {
    let rows = client.query(
        "SELECT
            i.relname as index_name,
            a.attname as column_name,
            ix.indisunique as is_unique
         FROM pg_index ix
         JOIN pg_class t ON t.oid = ix.indrelid
         JOIN pg_class i ON i.oid = ix.indexrelid
         JOIN pg_namespace n ON n.oid = t.relnamespace
         JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
         WHERE n.nspname = $1
           AND t.relname = $2
           AND NOT ix.indisprimary
         ORDER BY i.relname, a.attnum",
        &[&schema_name, &table_name],
    ).await?;

    let mut index_map: HashMap<String, Index> = HashMap::new();
    for row in &rows {
        let name: String = row.get::<_, &str>("index_name").to_string();
        let col: String = row.get::<_, &str>("column_name").to_string();
        let is_unique: bool = row.get("is_unique");

        let index = index_map.entry(name.clone()).or_insert_with(|| Index {
            name,
            columns: vec![],
            is_unique,
        });
        index.columns.push(col);
    }

    Ok(index_map.into_values().collect())
}

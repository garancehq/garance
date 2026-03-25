use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Schema {
    pub tables: HashMap<String, Table>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Table {
    pub name: String,
    pub schema: String,
    pub columns: Vec<Column>,
    pub primary_key: Option<Vec<String>>,
    pub foreign_keys: Vec<ForeignKey>,
    pub indexes: Vec<Index>,
}

impl Table {
    pub fn column(&self, name: &str) -> Option<&Column> {
        self.columns.iter().find(|c| c.name == name)
    }

    pub fn pk_columns(&self) -> Vec<&Column> {
        match &self.primary_key {
            Some(pk) => pk.iter().filter_map(|name| self.column(name)).collect(),
            None => vec![],
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Column {
    pub name: String,
    pub data_type: PgType,
    pub is_nullable: bool,
    pub has_default: bool,
    pub default_value: Option<String>,
    pub is_primary_key: bool,
    pub is_unique: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum PgType {
    Uuid, Text, Int4, Int8, Float8, Bool, Timestamp, Timestamptz,
    Date, Jsonb, Json, Bytea, Numeric, Serial, BigSerial, Other(String),
}

impl PgType {
    pub fn from_pg(type_name: &str) -> Self {
        match type_name {
            "uuid" => PgType::Uuid,
            "text" | "varchar" | "character varying" | "char" | "character" => PgType::Text,
            "int4" | "integer" | "int" => PgType::Int4,
            "int8" | "bigint" => PgType::Int8,
            "float8" | "double precision" => PgType::Float8,
            "bool" | "boolean" => PgType::Bool,
            "timestamp" | "timestamp without time zone" => PgType::Timestamp,
            "timestamptz" | "timestamp with time zone" => PgType::Timestamptz,
            "date" => PgType::Date,
            "jsonb" => PgType::Jsonb,
            "json" => PgType::Json,
            "bytea" => PgType::Bytea,
            "numeric" | "decimal" => PgType::Numeric,
            "serial" => PgType::Serial,
            "bigserial" => PgType::BigSerial,
            other => PgType::Other(other.to_string()),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ForeignKey {
    pub columns: Vec<String>,
    pub referenced_table: String,
    pub referenced_columns: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Index {
    pub name: String,
    pub columns: Vec<String>,
    pub is_unique: bool,
}

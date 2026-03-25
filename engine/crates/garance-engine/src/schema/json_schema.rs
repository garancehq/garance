use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceSchema {
    pub version: u32,
    pub tables: HashMap<String, GaranceTable>,
    #[serde(default)]
    pub storage: HashMap<String, GaranceBucket>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceTable {
    pub columns: HashMap<String, GaranceColumn>,
    #[serde(default)]
    pub relations: HashMap<String, GaranceRelation>,
    #[serde(default)]
    pub access: Option<GaranceAccess>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceColumn {
    #[serde(rename = "type")]
    pub col_type: String,
    #[serde(default)]
    pub primary_key: bool,
    #[serde(default)]
    pub unique: bool,
    #[serde(default)]
    pub nullable: bool,
    #[serde(default)]
    pub default: Option<String>,
    #[serde(default)]
    pub references: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceRelation {
    #[serde(rename = "type")]
    pub rel_type: String,
    pub table: String,
    pub foreign_key: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceAccess {
    #[serde(default)]
    pub read: Option<AccessRule>,
    #[serde(default)]
    pub write: Option<AccessRule>,
    #[serde(default)]
    pub delete: Option<AccessRule>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum AccessRule {
    Public(String),
    Conditions(Vec<AccessCondition>),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AccessCondition {
    #[serde(rename = "type")]
    pub condition_type: String,
    #[serde(default)]
    pub column: Option<String>,
    #[serde(default)]
    pub filters: Option<HashMap<String, serde_json::Value>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaranceBucket {
    #[serde(default)]
    pub max_file_size: Option<String>,
    #[serde(default)]
    pub allowed_mime_types: Option<Vec<String>>,
    #[serde(default)]
    pub access: Option<GaranceAccess>,
}

pub fn load_schema(path: &std::path::Path) -> Result<GaranceSchema, SchemaLoadError> {
    let content = std::fs::read_to_string(path).map_err(|e| SchemaLoadError::Io(e.to_string()))?;
    serde_json::from_str(&content).map_err(|e| SchemaLoadError::Parse(e.to_string()))
}

pub fn load_schema_from_str(json: &str) -> Result<GaranceSchema, SchemaLoadError> {
    serde_json::from_str(json).map_err(|e| SchemaLoadError::Parse(e.to_string()))
}

#[derive(Debug, thiserror::Error)]
pub enum SchemaLoadError {
    #[error("failed to read schema file: {0}")]
    Io(String),
    #[error("failed to parse schema JSON: {0}")]
    Parse(String),
}

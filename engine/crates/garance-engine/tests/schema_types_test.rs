use garance_engine::schema::{PgType, Column, Table, Schema};
use std::collections::HashMap;

#[test]
fn test_pg_type_from_pg() {
    assert_eq!(PgType::from_pg("uuid"), PgType::Uuid);
    assert_eq!(PgType::from_pg("text"), PgType::Text);
    assert_eq!(PgType::from_pg("varchar"), PgType::Text);
    assert_eq!(PgType::from_pg("character varying"), PgType::Text);
    assert_eq!(PgType::from_pg("integer"), PgType::Int4);
    assert_eq!(PgType::from_pg("boolean"), PgType::Bool);
    assert_eq!(PgType::from_pg("timestamp with time zone"), PgType::Timestamptz);
    assert_eq!(PgType::from_pg("jsonb"), PgType::Jsonb);
    assert_eq!(PgType::from_pg("custom_type"), PgType::Other("custom_type".into()));
}

#[test]
fn test_table_column_lookup() {
    let table = Table {
        name: "users".into(),
        schema: "public".into(),
        columns: vec![
            Column {
                name: "id".into(),
                data_type: PgType::Uuid,
                is_nullable: false,
                has_default: true,
                default_value: Some("gen_random_uuid()".into()),
                is_primary_key: true,
                is_unique: true,
            },
            Column {
                name: "email".into(),
                data_type: PgType::Text,
                is_nullable: false,
                has_default: false,
                default_value: None,
                is_primary_key: false,
                is_unique: true,
            },
        ],
        primary_key: Some(vec!["id".into()]),
        foreign_keys: vec![],
        indexes: vec![],
    };

    assert!(table.column("id").is_some());
    assert!(table.column("email").is_some());
    assert!(table.column("nonexistent").is_none());
    assert_eq!(table.pk_columns().len(), 1);
    assert_eq!(table.pk_columns()[0].name, "id");
}

#[test]
fn test_schema_serialization_roundtrip() {
    let mut tables = HashMap::new();
    tables.insert("users".into(), Table {
        name: "users".into(),
        schema: "public".into(),
        columns: vec![],
        primary_key: None,
        foreign_keys: vec![],
        indexes: vec![],
    });

    let schema = Schema { tables };
    let json = serde_json::to_string(&schema).unwrap();
    let deserialized: Schema = serde_json::from_str(&json).unwrap();
    assert_eq!(schema, deserialized);
}

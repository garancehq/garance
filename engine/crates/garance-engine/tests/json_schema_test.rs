use garance_engine::schema::json_schema::*;

#[test]
fn test_load_minimal_schema() {
    let json = r#"{"version": 1, "tables": {"users": {"columns": {"id": {"type": "uuid", "primary_key": true, "default": "gen_random_uuid()"}, "email": {"type": "text", "unique": true, "nullable": false}}}}}"#;
    let schema = load_schema_from_str(json).unwrap();
    assert_eq!(schema.version, 1);
    assert_eq!(schema.tables.len(), 1);
    let users = schema.tables.get("users").unwrap();
    assert_eq!(users.columns.len(), 2);
    let id_col = users.columns.get("id").unwrap();
    assert!(id_col.primary_key);
    assert_eq!(id_col.col_type, "uuid");
    assert_eq!(id_col.default, Some("gen_random_uuid()".into()));
}

#[test]
fn test_load_schema_with_relations() {
    let json = r#"{"version": 1, "tables": {"users": {"columns": {"id": {"type": "uuid", "primary_key": true}}, "relations": {"posts": {"type": "hasMany", "table": "posts", "foreign_key": "author_id"}}}, "posts": {"columns": {"id": {"type": "uuid", "primary_key": true}, "author_id": {"type": "uuid", "references": "users.id"}}}}}"#;
    let schema = load_schema_from_str(json).unwrap();
    let users = schema.tables.get("users").unwrap();
    let rel = users.relations.get("posts").unwrap();
    assert_eq!(rel.rel_type, "hasMany");
    assert_eq!(rel.table, "posts");
    assert_eq!(rel.foreign_key, "author_id");
}

#[test]
fn test_load_schema_with_access_rules() {
    let json = r#"{"version": 1, "tables": {"posts": {"columns": {"id": {"type": "uuid", "primary_key": true}, "published": {"type": "bool"}, "author_id": {"type": "uuid"}}, "access": {"read": [{"type": "where", "filters": {"published": true}}, {"type": "isOwner", "column": "author_id"}], "write": [{"type": "isOwner", "column": "author_id"}]}}}}"#;
    let schema = load_schema_from_str(json).unwrap();
    let posts = schema.tables.get("posts").unwrap();
    let access = posts.access.as_ref().unwrap();
    assert!(access.read.is_some());
    assert!(access.write.is_some());
    assert!(access.delete.is_none());
}

#[test]
fn test_load_schema_with_storage() {
    let json = r#"{"version": 1, "tables": {}, "storage": {"avatars": {"max_file_size": "5mb", "allowed_mime_types": ["image/jpeg", "image/png"], "access": {"read": "public", "write": [{"type": "isAuthenticated"}]}}}}"#;
    let schema = load_schema_from_str(json).unwrap();
    assert_eq!(schema.storage.len(), 1);
    let avatars = schema.storage.get("avatars").unwrap();
    assert_eq!(avatars.max_file_size, Some("5mb".into()));
    assert_eq!(avatars.allowed_mime_types, Some(vec!["image/jpeg".into(), "image/png".into()]));
}

#[test]
fn test_load_invalid_json_returns_error() {
    let result = load_schema_from_str("not json");
    assert!(result.is_err());
}

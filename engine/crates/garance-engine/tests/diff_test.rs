use std::collections::HashMap;
use garance_engine::diff::diff;
use garance_engine::schema::json_schema::{GaranceSchema, GaranceTable, GaranceColumn, GaranceRelation};
use garance_engine::schema::types::{Schema, Table, Column, PgType, ForeignKey, Index};

/// Helper: build an empty GaranceSchema.
fn empty_desired() -> GaranceSchema {
    GaranceSchema {
        version: 1,
        tables: HashMap::new(),
        storage: HashMap::new(),
    }
}

/// Helper: build an empty Schema (current PG state).
fn empty_current() -> Schema {
    Schema {
        tables: HashMap::new(),
    }
}

/// Helper: build a GaranceColumn with defaults.
fn gcol(col_type: &str, primary_key: bool, nullable: bool, unique: bool, default: Option<&str>, references: Option<&str>) -> GaranceColumn {
    GaranceColumn {
        col_type: col_type.to_string(),
        primary_key,
        unique,
        nullable,
        default: default.map(|s| s.to_string()),
        references: references.map(|s| s.to_string()),
    }
}

/// Helper: build a Column (from PG introspection).
fn pg_col(name: &str, data_type: PgType, is_nullable: bool, is_pk: bool, is_unique: bool, default: Option<&str>) -> Column {
    Column {
        name: name.to_string(),
        data_type,
        is_nullable,
        has_default: default.is_some(),
        default_value: default.map(|s| s.to_string()),
        is_primary_key: is_pk,
        is_unique,
    }
}

/// Helper: build a PG Table.
fn pg_table(name: &str, columns: Vec<Column>, pk: Option<Vec<&str>>, fks: Vec<ForeignKey>, indexes: Vec<Index>) -> Table {
    Table {
        name: name.to_string(),
        schema: "public".to_string(),
        columns,
        primary_key: pk.map(|v| v.into_iter().map(|s| s.to_string()).collect()),
        foreign_keys: fks,
        indexes,
    }
}

#[test]
fn test_diff_create_table() {
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, Some("gen_random_uuid()"), None));
            cols.insert("email".to_string(), gcol("text", false, false, true, None, None));
            cols.insert("name".to_string(), gcol("text", false, true, false, None, None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let current = empty_current();
    let result = diff(&desired, &current);

    assert_eq!(result.summary.tables_created, 1);
    assert!(!result.has_destructive);
    assert!(!result.statements.is_empty());

    // Should have a CREATE TABLE statement
    let create_stmt = result.statements.iter().find(|s| s.starts_with("CREATE TABLE")).unwrap();
    assert!(create_stmt.contains("\"users\""));
    assert!(create_stmt.contains("\"id\""));
    assert!(create_stmt.contains("\"email\""));
    assert!(create_stmt.contains("\"name\""));
    assert!(create_stmt.contains("PRIMARY KEY"));

    // Should have a UNIQUE constraint for email (non-PK unique column)
    let unique_stmt = result.statements.iter().find(|s| s.contains("UNIQUE"));
    assert!(unique_stmt.is_some(), "Expected a UNIQUE constraint for email");
    let unique_stmt = unique_stmt.unwrap();
    assert!(unique_stmt.contains("users_email_key"));
    assert!(unique_stmt.contains("UNIQUE (\"email\")"));
}

#[test]
fn test_diff_drop_table() {
    let desired = empty_desired();

    let mut current = empty_current();
    current.tables.insert("old_table".to_string(), pg_table(
        "old_table",
        vec![pg_col("id", PgType::Uuid, false, true, false, None)],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    assert_eq!(result.summary.tables_dropped, 1);
    assert!(result.has_destructive);
    assert!(result.statements.iter().any(|s| s == "DROP TABLE \"old_table\""));
}

#[test]
fn test_diff_add_column() {
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("email".to_string(), gcol("text", false, false, false, None, None));
            cols.insert("bio".to_string(), gcol("text", false, true, false, None, None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("users".to_string(), pg_table(
        "users",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, None),
            pg_col("email", PgType::Text, false, false, false, None),
        ],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    assert_eq!(result.summary.columns_added, 1);
    assert!(result.statements.iter().any(|s| s.contains("ADD COLUMN \"bio\"")));
}

#[test]
fn test_diff_drop_column() {
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("users".to_string(), pg_table(
        "users",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, None),
            pg_col("email", PgType::Text, false, false, false, None),
        ],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    assert_eq!(result.summary.columns_dropped, 1);
    assert!(result.has_destructive);
    assert!(result.statements.iter().any(|s| s.contains("DROP COLUMN \"email\"")));
}

#[test]
fn test_diff_change_type() {
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("age".to_string(), gcol("int8", false, false, false, None, None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("users".to_string(), pg_table(
        "users",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, None),
            pg_col("age", PgType::Int4, false, false, false, None),
        ],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    assert_eq!(result.summary.columns_modified, 1);
    assert!(result.statements.iter().any(|s| s.contains("ALTER COLUMN \"age\" TYPE int8")));
}

#[test]
fn test_diff_change_nullable() {
    // Make a non-nullable column nullable
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("email".to_string(), gcol("text", false, true, false, None, None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("users".to_string(), pg_table(
        "users",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, None),
            pg_col("email", PgType::Text, false, false, false, None),
        ],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    assert_eq!(result.summary.columns_modified, 1);
    assert!(result.statements.iter().any(|s| s.contains("DROP NOT NULL")));
}

#[test]
fn test_diff_change_default() {
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, Some("gen_random_uuid()"), None));
            cols.insert("active".to_string(), gcol("bool", false, false, false, Some("true"), None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("users".to_string(), pg_table(
        "users",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, Some("gen_random_uuid()")),
            pg_col("active", PgType::Bool, false, false, false, Some("false")),
        ],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    assert!(result.summary.columns_modified >= 1);
    assert!(result.statements.iter().any(|s| s.contains("SET DEFAULT true")));
}

#[test]
fn test_diff_add_foreign_key() {
    let mut desired = empty_desired();
    desired.tables.insert("posts".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("user_id".to_string(), gcol("uuid", false, false, false, None, None));
            cols
        },
        relations: {
            let mut rels = HashMap::new();
            rels.insert("author".to_string(), GaranceRelation {
                rel_type: "many_to_one".to_string(),
                table: "users".to_string(),
                foreign_key: "user_id".to_string(),
            });
            rels
        },
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("posts".to_string(), pg_table(
        "posts",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, None),
            pg_col("user_id", PgType::Uuid, false, false, false, None),
        ],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    assert_eq!(result.summary.foreign_keys_added, 1);
    assert!(result.statements.iter().any(|s| {
        s.contains("ADD CONSTRAINT \"fk_posts_author\"")
            && s.contains("FOREIGN KEY")
            && s.contains("REFERENCES \"users\"")
    }));
}

#[test]
fn test_diff_drop_foreign_key() {
    let mut desired = empty_desired();
    desired.tables.insert("posts".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("user_id".to_string(), gcol("uuid", false, false, false, None, None));
            cols
        },
        relations: HashMap::new(), // no relations desired
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("posts".to_string(), pg_table(
        "posts",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, None),
            pg_col("user_id", PgType::Uuid, false, false, false, None),
        ],
        Some(vec!["id"]),
        vec![ForeignKey {
            constraint_name: "fk_posts_author".to_string(),
            columns: vec!["user_id".to_string()],
            referenced_table: "users".to_string(),
            referenced_columns: vec!["id".to_string()],
        }],
        vec![],
    ));

    let result = diff(&desired, &current);

    assert_eq!(result.summary.foreign_keys_dropped, 1);
    assert!(result.statements.iter().any(|s| {
        s.contains("DROP CONSTRAINT \"fk_posts_author\"")
    }));
}

#[test]
fn test_diff_add_unique() {
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("email".to_string(), gcol("text", false, false, true, None, None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let current = empty_current();
    let result = diff(&desired, &current);

    // Should create the table AND add a UNIQUE constraint
    assert_eq!(result.summary.tables_created, 1);
    let unique_stmt = result.statements.iter().find(|s| s.contains("UNIQUE"));
    assert!(unique_stmt.is_some());
    let unique_stmt = unique_stmt.unwrap();
    assert!(unique_stmt.contains("users_email_key"));
    assert!(unique_stmt.contains("UNIQUE (\"email\")"));
}

#[test]
fn test_diff_no_changes() {
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("email".to_string(), gcol("text", false, false, false, None, None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("users".to_string(), pg_table(
        "users",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, None),
            pg_col("email", PgType::Text, false, false, false, None),
        ],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    assert!(result.statements.is_empty());
    assert!(!result.has_destructive);
    assert_eq!(result.summary.tables_created, 0);
    assert_eq!(result.summary.tables_dropped, 0);
    assert_eq!(result.summary.columns_added, 0);
    assert_eq!(result.summary.columns_dropped, 0);
    assert_eq!(result.summary.columns_modified, 0);
}

#[test]
fn test_diff_statement_ordering() {
    // Create a new table with FK → CREATE TABLE must come before ADD CONSTRAINT
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });
    desired.tables.insert("posts".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("user_id".to_string(), gcol("uuid", false, false, false, None, None));
            cols
        },
        relations: {
            let mut rels = HashMap::new();
            rels.insert("author".to_string(), GaranceRelation {
                rel_type: "many_to_one".to_string(),
                table: "users".to_string(),
                foreign_key: "user_id".to_string(),
            });
            rels
        },
        access: None,
    });

    let current = empty_current();
    let result = diff(&desired, &current);

    // CREATE TABLE statements should come before FK constraints
    let create_positions: Vec<usize> = result.statements.iter().enumerate()
        .filter(|(_, s)| s.starts_with("CREATE TABLE"))
        .map(|(i, _)| i)
        .collect();
    let fk_positions: Vec<usize> = result.statements.iter().enumerate()
        .filter(|(_, s)| s.contains("FOREIGN KEY"))
        .map(|(i, _)| i)
        .collect();

    if !fk_positions.is_empty() {
        let max_create = *create_positions.iter().max().unwrap();
        let min_fk = *fk_positions.iter().min().unwrap();
        assert!(max_create < min_fk, "CREATE TABLE must come before FOREIGN KEY constraints");
    }
}

#[test]
fn test_diff_destructive_flag() {
    // Dropping a table sets has_destructive
    let desired = empty_desired();
    let mut current = empty_current();
    current.tables.insert("old".to_string(), pg_table(
        "old",
        vec![pg_col("id", PgType::Uuid, false, true, false, None)],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);
    assert!(result.has_destructive);

    // No changes = not destructive
    let result2 = diff(&empty_desired(), &empty_current());
    assert!(!result2.has_destructive);
}

#[test]
fn test_diff_type_normalization() {
    // varchar in desired should match text in current (no false diff)
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("email".to_string(), gcol("varchar", false, false, false, None, None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("users".to_string(), pg_table(
        "users",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, None),
            pg_col("email", PgType::Text, false, false, false, None),
        ],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    // varchar normalizes to text, text normalizes to text → no diff
    assert!(result.statements.is_empty(), "varchar vs text should produce no diff, got: {:?}", result.statements);
    assert_eq!(result.summary.columns_modified, 0);
}

#[test]
fn test_diff_default_normalization() {
    // now() in desired should match CURRENT_TIMESTAMP in current
    let mut desired = empty_desired();
    desired.tables.insert("users".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, None, None));
            cols.insert("created_at".to_string(), gcol("timestamptz", false, false, false, Some("now()"), None));
            cols
        },
        relations: HashMap::new(),
        access: None,
    });

    let mut current = empty_current();
    current.tables.insert("users".to_string(), pg_table(
        "users",
        vec![
            pg_col("id", PgType::Uuid, false, true, false, None),
            pg_col("created_at", PgType::Timestamptz, false, false, false, Some("CURRENT_TIMESTAMP")),
        ],
        Some(vec!["id"]),
        vec![],
        vec![],
    ));

    let result = diff(&desired, &current);

    // now() normalizes same as CURRENT_TIMESTAMP → no diff
    assert!(result.statements.is_empty(), "now() vs CURRENT_TIMESTAMP should produce no diff, got: {:?}", result.statements);
    assert_eq!(result.summary.columns_modified, 0);
}

#[test]
fn test_diff_self_referential_fk() {
    // A table referencing itself — FK should be deferred to ALTER TABLE (not inlined)
    let mut desired = empty_desired();
    desired.tables.insert("categories".to_string(), GaranceTable {
        columns: {
            let mut cols = HashMap::new();
            cols.insert("id".to_string(), gcol("uuid", true, false, false, Some("gen_random_uuid()"), None));
            cols.insert("name".to_string(), gcol("text", false, false, false, None, None));
            cols.insert("parent_id".to_string(), gcol("uuid", false, true, false, None, None));
            cols
        },
        relations: {
            let mut rels = HashMap::new();
            rels.insert("parent".to_string(), GaranceRelation {
                rel_type: "many_to_one".to_string(),
                table: "categories".to_string(),
                foreign_key: "parent_id".to_string(),
            });
            rels
        },
        access: None,
    });

    let current = empty_current();
    let result = diff(&desired, &current);

    assert_eq!(result.summary.tables_created, 1);
    assert_eq!(result.summary.foreign_keys_added, 1);

    // CREATE TABLE must come before the self-referential FK
    let create_pos = result.statements.iter().position(|s| s.starts_with("CREATE TABLE")).unwrap();
    let fk_pos = result.statements.iter().position(|s| s.contains("FOREIGN KEY")).unwrap();
    assert!(create_pos < fk_pos, "CREATE TABLE must come before self-referential FK");

    // FK should reference the same table
    let fk_stmt = result.statements.iter().find(|s| s.contains("FOREIGN KEY")).unwrap();
    assert!(fk_stmt.contains("REFERENCES \"categories\""), "Self-referential FK should reference 'categories'");
    assert!(fk_stmt.contains("fk_categories_parent"));
}

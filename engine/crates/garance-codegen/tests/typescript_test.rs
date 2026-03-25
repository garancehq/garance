use garance_codegen::typescript::*;

#[test]
fn test_pg_type_mapping() {
    assert_eq!(pg_type_to_ts("Uuid"), "string");
    assert_eq!(pg_type_to_ts("Text"), "string");
    assert_eq!(pg_type_to_ts("Int4"), "number");
    assert_eq!(pg_type_to_ts("Bool"), "boolean");
    assert_eq!(pg_type_to_ts("Jsonb"), "unknown");
}

#[test]
fn test_generate_simple_types() {
    let tables = vec![
        TableDef {
            name: "users".into(),
            columns: vec![
                ColumnDef { name: "id".into(), data_type: "Uuid".into(), is_nullable: false, has_default: true },
                ColumnDef { name: "email".into(), data_type: "Text".into(), is_nullable: false, has_default: false },
                ColumnDef { name: "name".into(), data_type: "Text".into(), is_nullable: false, has_default: false },
                ColumnDef { name: "bio".into(), data_type: "Text".into(), is_nullable: true, has_default: false },
            ],
        }
    ];
    let output = generate_types(&tables);
    assert!(output.contains("export interface Users {"));
    assert!(output.contains("  id: string;"));
    assert!(output.contains("  email: string;"));
    assert!(output.contains("  bio: string | null;"));
    assert!(output.contains("export interface UsersInsert {"));
    assert!(output.contains("  id?: string;"));
    assert!(output.contains("  bio?: string | null;"));
    assert!(output.contains("export interface UsersUpdate {"));
    assert!(output.contains("  id?: string;"));
    assert!(output.contains("  email?: string;"));
}

#[test]
fn test_generate_multiple_tables() {
    let tables = vec![
        TableDef { name: "users".into(), columns: vec![] },
        TableDef { name: "blog_posts".into(), columns: vec![] },
    ];
    let output = generate_types(&tables);
    assert!(output.contains("export interface Users {"));
    assert!(output.contains("export interface BlogPosts {"));
}

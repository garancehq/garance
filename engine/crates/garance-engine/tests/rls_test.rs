use garance_engine::schema::rls::*;
use garance_engine::schema::json_schema::*;
use std::collections::HashMap;

fn make_access_isowner(column: &str) -> GaranceAccess {
    GaranceAccess {
        read: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "isOwner".into(),
            column: Some(column.into()),
            filters: None,
        }])),
        write: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "isOwner".into(),
            column: Some(column.into()),
            filters: None,
        }])),
        delete: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "isOwner".into(),
            column: Some(column.into()),
            filters: None,
        }])),
    }
}

#[test]
fn test_rls_policy_isowner() {
    let access = make_access_isowner("author_id");
    let policies = generate_policies("posts", &access);
    assert_eq!(policies.len(), 4); // select, insert, update, delete

    let select = &policies[0].1;
    assert!(select.contains("FOR SELECT USING"));
    assert!(select.contains("\"author_id\"::text = current_setting('request.user_id', true)"));
}

#[test]
fn test_rls_policy_where() {
    let mut filters = HashMap::new();
    filters.insert("published".into(), serde_json::Value::Bool(true));

    let access = GaranceAccess {
        read: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "where".into(),
            column: None,
            filters: Some(filters),
        }])),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    assert_eq!(policies.len(), 1);
    assert!(policies[0].1.contains("\"published\" = true"));
}

#[test]
fn test_rls_policy_public() {
    let access = GaranceAccess {
        read: Some(AccessRule::Public("public".into())),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    assert!(policies[0].1.contains("USING (true)"));
}

#[test]
fn test_rls_policy_authenticated() {
    let access = GaranceAccess {
        read: Some(AccessRule::Public("authenticated".into())),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    assert!(policies[0].1.contains("IS NOT NULL"));
}

#[test]
fn test_rls_policy_combined_or() {
    let mut filters = HashMap::new();
    filters.insert("published".into(), serde_json::Value::Bool(true));

    let access = GaranceAccess {
        read: Some(AccessRule::Conditions(vec![
            AccessCondition { condition_type: "where".into(), column: None, filters: Some(filters) },
            AccessCondition { condition_type: "isOwner".into(), column: Some("author_id".into()), filters: None },
        ])),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    let sql = &policies[0].1;
    assert!(sql.contains(" OR "));
    assert!(sql.contains("\"published\" = true"));
    assert!(sql.contains("\"author_id\"::text"));
}

#[test]
fn test_rls_update_has_with_check() {
    let access = make_access_isowner("author_id");
    let policies = generate_policies("posts", &access);
    let update = policies.iter().find(|(name, _)| name.contains("update")).unwrap();
    assert!(update.1.contains("USING ("));
    assert!(update.1.contains("WITH CHECK ("));
}

#[test]
fn test_rls_delete_not_inherited() {
    let access = GaranceAccess {
        read: Some(AccessRule::Public("public".into())),
        write: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "isOwner".into(),
            column: Some("author_id".into()),
            filters: None,
        }])),
        delete: None, // NOT specified
    };

    let policies = generate_policies("posts", &access);
    let has_delete = policies.iter().any(|(name, _)| name.contains("delete"));
    assert!(!has_delete, "delete policy should NOT exist when not declared");
}

#[test]
fn test_enable_rls() {
    let stmts = enable_rls("posts");
    assert_eq!(stmts.len(), 2);
    assert!(stmts[0].contains("ENABLE ROW LEVEL SECURITY"));
    assert!(stmts[1].contains("FORCE ROW LEVEL SECURITY"));
}

#[test]
fn test_value_escaping() {
    let mut filters = HashMap::new();
    filters.insert("name".into(), serde_json::Value::String("O'Brien".into()));

    let access = GaranceAccess {
        read: Some(AccessRule::Conditions(vec![AccessCondition {
            condition_type: "where".into(),
            column: None,
            filters: Some(filters),
        }])),
        write: None,
        delete: None,
    };

    let policies = generate_policies("posts", &access);
    assert!(policies[0].1.contains("'O''Brien'"), "single quotes should be doubled");
}

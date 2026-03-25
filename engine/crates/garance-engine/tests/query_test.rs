use garance_engine::query::filter::*;
use garance_engine::query::builder::*;
use garance_engine::schema::types::*;

fn test_table() -> Table {
    Table {
        name: "users".into(), schema: "public".into(),
        columns: vec![
            Column { name: "id".into(), data_type: PgType::Uuid, is_nullable: false, has_default: true, default_value: None, is_primary_key: true, is_unique: true },
            Column { name: "name".into(), data_type: PgType::Text, is_nullable: false, has_default: false, default_value: None, is_primary_key: false, is_unique: false },
            Column { name: "age".into(), data_type: PgType::Int4, is_nullable: true, has_default: false, default_value: None, is_primary_key: false, is_unique: false },
            Column { name: "email".into(), data_type: PgType::Text, is_nullable: false, has_default: false, default_value: None, is_primary_key: false, is_unique: true },
        ],
        primary_key: Some(vec!["id".into()]), foreign_keys: vec![], indexes: vec![],
    }
}

#[test]
fn test_parse_simple_filter() {
    let params = vec![("age".into(), "gte.18".into()), ("name".into(), "eq.Alice".into())];
    let qp = parse_query_params(&params).unwrap();
    assert_eq!(qp.filters.len(), 2);
    assert_eq!(qp.filters[0].column, "age");
    assert_eq!(qp.filters[0].operator, Operator::Gte);
    assert_eq!(qp.filters[0].value, "18");
}

#[test]
fn test_parse_select_and_order() {
    let params = vec![("select".into(), "id,name,email".into()), ("order".into(), "name.asc,age.desc".into()), ("limit".into(), "20".into())];
    let qp = parse_query_params(&params).unwrap();
    assert_eq!(qp.select, Some(vec!["id".into(), "name".into(), "email".into()]));
    assert_eq!(qp.order.len(), 2);
    assert_eq!(qp.order[0].column, "name");
    assert_eq!(qp.order[0].direction, SortDirection::Asc);
    assert_eq!(qp.order[1].direction, SortDirection::Desc);
    assert_eq!(qp.limit, Some(20));
}

#[test]
fn test_build_select_simple() {
    let table = test_table();
    let qp = QueryParams::default();
    let result = build_select(&table, &qp).unwrap();
    assert_eq!(result.sql, "SELECT * FROM \"users\"");
    assert!(result.params.is_empty());
}

#[test]
fn test_build_select_with_filters() {
    let table = test_table();
    let qp = QueryParams { filters: vec![Filter { column: "age".into(), operator: Operator::Gte, value: "18".into() }], limit: Some(10), ..Default::default() };
    let result = build_select(&table, &qp).unwrap();
    assert_eq!(result.sql, "SELECT * FROM \"users\" WHERE \"age\"::text >= $1 LIMIT 10");
    assert_eq!(result.params, vec!["18"]);
}

#[test]
fn test_build_select_unknown_column_rejected() {
    let table = test_table();
    let qp = QueryParams { select: Some(vec!["nonexistent".into()]), ..Default::default() };
    assert!(build_select(&table, &qp).is_err());
}

#[test]
fn test_build_select_by_id() {
    let table = test_table();
    let result = build_select_by_id(&table, "abc-123").unwrap();
    assert_eq!(result.sql, "SELECT * FROM \"users\" WHERE \"id\"::text = $1");
    assert_eq!(result.params, vec!["abc-123"]);
}

#[test]
fn test_build_insert() {
    let table = test_table();
    let cols = vec!["name".into(), "email".into()];
    let result = build_insert(&table, &cols).unwrap();
    assert_eq!(result.sql, "INSERT INTO \"users\" (\"name\", \"email\") VALUES ($1, $2) RETURNING *");
}

#[test]
fn test_build_update() {
    let table = test_table();
    let cols = vec!["name".into()];
    let result = build_update(&table, "abc-123", &cols).unwrap();
    assert_eq!(result.sql, "UPDATE \"users\" SET \"name\" = $1 WHERE \"id\"::text = $2 RETURNING *");
}

#[test]
fn test_build_delete() {
    let table = test_table();
    let result = build_delete(&table, "abc-123").unwrap();
    assert_eq!(result.sql, "DELETE FROM \"users\" WHERE \"id\"::text = $1");
}

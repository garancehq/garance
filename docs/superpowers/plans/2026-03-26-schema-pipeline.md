# Schema Pipeline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the schema pipeline — compile `garance.schema.ts` to JSON, diff against PostgreSQL, generate and apply SQL migrations. The core differentiator of Garance.

**Architecture:** 5 tasks: (1) fix the `ForeignKey` model to include constraint names, (2) build the diff engine as a new Rust module, (3) add Engine API endpoints for preview/apply, (4) update `@garance/schema` for `renamedFrom`, (5) wire the CLI to orchestrate the pipeline. Each task produces testable, working code independently.

**Tech Stack:** Rust (diff engine + API), TypeScript (@garance/schema), Go (CLI), sha2 crate (checksum)

**Spec:** `docs/superpowers/specs/2026-03-26-schema-pipeline-design.md`

---

## Task 1: Fix ForeignKey Model + Introspection

The `ForeignKey` struct in `types.rs` lacks a `constraint_name` field. Without it, the diff engine cannot generate `DROP CONSTRAINT` statements.

**Files:**
- Modify: `engine/crates/garance-engine/src/schema/types.rs` — add `constraint_name` to `ForeignKey`
- Modify: `engine/crates/garance-engine/src/schema/introspect.rs` — store constraint name
- Modify: `engine/crates/garance-engine/tests/introspect_test.rs` — verify constraint name

- [ ] **Step 1: Add `constraint_name` to ForeignKey**

In `engine/crates/garance-engine/src/schema/types.rs`, change:

```rust
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ForeignKey {
    pub constraint_name: String,
    pub columns: Vec<String>,
    pub referenced_table: String,
    pub referenced_columns: Vec<String>,
}
```

- [ ] **Step 2: Store constraint_name in introspection**

In `engine/crates/garance-engine/src/schema/introspect.rs`, in `introspect_foreign_keys`, change the `or_insert_with` to include the constraint name:

```rust
let fk = fks_by_constraint.entry(constraint.clone()).or_insert_with(|| ForeignKey {
    constraint_name: constraint,
    columns: vec![],
    referenced_table: ref_table,
    referenced_columns: vec![],
});
```

- [ ] **Step 3: Update FK test assertion**

In `engine/crates/garance-engine/tests/introspect_test.rs`, in `test_introspect_foreign_keys`, add:

```rust
assert!(!posts.foreign_keys[0].constraint_name.is_empty());
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/engine && cargo test -p garance-engine`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add engine/
git commit -m ":sparkles: feat(engine): add constraint_name to ForeignKey model"
```

---

## Task 2: Diff Engine — Core Module

The heart of the schema pipeline. New module `diff/` in `garance-engine` that compares a `GaranceSchema` (desired) with a `Schema` (current) and produces ordered SQL statements.

**Files:**
- Create: `engine/crates/garance-engine/src/diff/mod.rs`
- Create: `engine/crates/garance-engine/src/diff/normalize.rs` — type + default normalization
- Create: `engine/crates/garance-engine/src/diff/sql_gen.rs` — SQL statement generation
- Create: `engine/crates/garance-engine/src/diff/diff.rs` — main diff algorithm
- Modify: `engine/crates/garance-engine/src/lib.rs` — add `pub mod diff;`
- Create: `engine/crates/garance-engine/tests/diff_test.rs` — 13 diff tests

- [ ] **Step 1: Create normalize module**

```rust
// engine/crates/garance-engine/src/diff/normalize.rs

/// Normalize a column type string to its canonical PostgreSQL form.
/// Both GaranceColumn.col_type and PgType are normalized through this
/// before comparison.
pub fn normalize_type(t: &str) -> String {
    let s = t.trim().to_lowercase();
    match s.as_str() {
        "text" | "varchar" | "character varying" | "char" | "character" | "bpchar" => "text".into(),
        "int4" | "integer" | "int" => "int4".into(),
        "int2" | "smallint" => "int2".into(),
        "int8" | "bigint" => "int8".into(),
        "float4" | "real" => "float4".into(),
        "float8" | "double precision" => "float8".into(),
        "bool" | "boolean" => "bool".into(),
        "timestamp" | "timestamp without time zone" => "timestamp".into(),
        "timestamptz" | "timestamp with time zone" => "timestamptz".into(),
        "date" => "date".into(),
        "uuid" => "uuid".into(),
        "jsonb" => "jsonb".into(),
        "json" => "json".into(),
        "bytea" => "bytea".into(),
        "numeric" | "decimal" => "numeric".into(),
        "serial" => "int4".into(),      // serial is int4 + nextval default
        "bigserial" => "int8".into(),   // bigserial is int8 + nextval default
        other => other.to_lowercase(),
    }
}

/// Normalize a default expression to its canonical form.
pub fn normalize_default(d: &str) -> String {
    let trimmed = d.trim();
    let lower = trimmed.to_lowercase();

    // now() / CURRENT_TIMESTAMP
    if lower == "now()" || lower == "current_timestamp" || lower.starts_with("now()::") {
        return "now()".into();
    }

    // Boolean literals
    if lower == "true" || lower == "'t'::boolean" {
        return "true".into();
    }
    if lower == "false" || lower == "'f'::boolean" {
        return "false".into();
    }

    // gen_random_uuid()
    if lower == "gen_random_uuid()" {
        return "gen_random_uuid()".into();
    }

    // Pass through as-is
    trimmed.to_string()
}

/// Convert a PgType enum to its canonical string representation.
pub fn pg_type_to_canonical(pg_type: &crate::schema::types::PgType) -> String {
    use crate::schema::types::PgType;
    match pg_type {
        PgType::Uuid => "uuid".into(),
        PgType::Text => "text".into(),
        PgType::Int4 => "int4".into(),
        PgType::Int8 => "int8".into(),
        PgType::Float8 => "float8".into(),
        PgType::Bool => "bool".into(),
        PgType::Timestamp => "timestamp".into(),
        PgType::Timestamptz => "timestamptz".into(),
        PgType::Date => "date".into(),
        PgType::Jsonb => "jsonb".into(),
        PgType::Json => "json".into(),
        PgType::Bytea => "bytea".into(),
        PgType::Numeric => "numeric".into(),
        PgType::Serial => "int4".into(),
        PgType::BigSerial => "int8".into(),
        PgType::Other(s) => s.to_lowercase(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_type_normalization() {
        assert_eq!(normalize_type("varchar"), "text");
        assert_eq!(normalize_type("character varying"), "text");
        assert_eq!(normalize_type("INTEGER"), "int4");
        assert_eq!(normalize_type("serial"), "int4");
        assert_eq!(normalize_type("bigserial"), "int8");
        assert_eq!(normalize_type("BOOLEAN"), "bool");
        assert_eq!(normalize_type("timestamp with time zone"), "timestamptz");
        assert_eq!(normalize_type("custom_type"), "custom_type");
    }

    #[test]
    fn test_default_normalization() {
        assert_eq!(normalize_default("now()"), "now()");
        assert_eq!(normalize_default("CURRENT_TIMESTAMP"), "now()");
        assert_eq!(normalize_default("true"), "true");
        assert_eq!(normalize_default("'f'::boolean"), "false");
        assert_eq!(normalize_default("gen_random_uuid()"), "gen_random_uuid()");
        assert_eq!(normalize_default("42"), "42");
    }
}
```

- [ ] **Step 2: Create SQL generation helpers**

```rust
// engine/crates/garance-engine/src/diff/sql_gen.rs

/// Generate a CREATE TABLE statement (without foreign keys — those are added separately).
pub fn create_table(table_name: &str, columns: &[(String, String, bool, Option<String>, bool, bool)]) -> String {
    // columns: (name, type, nullable, default, is_pk, is_unique)
    let mut parts = vec![];
    let mut pk_cols = vec![];

    for (name, col_type, nullable, default, is_pk, _is_unique) in columns {
        let mut col_def = format!("  \"{}\" {}", name, map_type_to_sql(col_type));
        if !nullable {
            col_def.push_str(" NOT NULL");
        }
        if let Some(def) = default {
            col_def.push_str(&format!(" DEFAULT {}", def));
        }
        if *is_pk {
            pk_cols.push(format!("\"{}\"", name));
        }
        parts.push(col_def);
    }

    if !pk_cols.is_empty() {
        parts.push(format!("  PRIMARY KEY ({})", pk_cols.join(", ")));
    }

    format!("CREATE TABLE \"{}\" (\n{}\n)", table_name, parts.join(",\n"))
}

pub fn drop_table(table_name: &str) -> String {
    format!("DROP TABLE \"{}\"", table_name)
}

pub fn add_column(table_name: &str, col_name: &str, col_type: &str, nullable: bool, default: Option<&str>) -> String {
    let mut sql = format!("ALTER TABLE \"{}\" ADD COLUMN \"{}\" {}", table_name, col_name, map_type_to_sql(col_type));
    if !nullable {
        sql.push_str(" NOT NULL");
    }
    if let Some(def) = default {
        sql.push_str(&format!(" DEFAULT {}", def));
    }
    sql
}

pub fn drop_column(table_name: &str, col_name: &str) -> String {
    format!("ALTER TABLE \"{}\" DROP COLUMN \"{}\"", table_name, col_name)
}

pub fn alter_column_type(table_name: &str, col_name: &str, new_type: &str) -> String {
    format!("ALTER TABLE \"{}\" ALTER COLUMN \"{}\" TYPE {} USING \"{}\"::{}", table_name, col_name, map_type_to_sql(new_type), col_name, map_type_to_sql(new_type))
}

pub fn set_not_null(table_name: &str, col_name: &str) -> String {
    format!("ALTER TABLE \"{}\" ALTER COLUMN \"{}\" SET NOT NULL", table_name, col_name)
}

pub fn drop_not_null(table_name: &str, col_name: &str) -> String {
    format!("ALTER TABLE \"{}\" ALTER COLUMN \"{}\" DROP NOT NULL", table_name, col_name)
}

pub fn set_default(table_name: &str, col_name: &str, default: &str) -> String {
    format!("ALTER TABLE \"{}\" ALTER COLUMN \"{}\" SET DEFAULT {}", table_name, col_name, default)
}

pub fn drop_default(table_name: &str, col_name: &str) -> String {
    format!("ALTER TABLE \"{}\" ALTER COLUMN \"{}\" DROP DEFAULT", table_name, col_name)
}

pub fn add_foreign_key(table_name: &str, constraint_name: &str, columns: &[String], ref_table: &str, ref_columns: &[String]) -> String {
    format!(
        "ALTER TABLE \"{}\" ADD CONSTRAINT \"{}\" FOREIGN KEY ({}) REFERENCES \"{}\" ({})",
        table_name, constraint_name,
        columns.iter().map(|c| format!("\"{}\"", c)).collect::<Vec<_>>().join(", "),
        ref_table,
        ref_columns.iter().map(|c| format!("\"{}\"", c)).collect::<Vec<_>>().join(", "),
    )
}

pub fn drop_constraint(table_name: &str, constraint_name: &str) -> String {
    format!("ALTER TABLE \"{}\" DROP CONSTRAINT \"{}\"", table_name, constraint_name)
}

pub fn create_index(index_name: &str, table_name: &str, columns: &[String], is_unique: bool) -> String {
    let unique = if is_unique { "UNIQUE " } else { "" };
    format!(
        "CREATE {}INDEX \"{}\" ON \"{}\" ({})",
        unique, index_name, table_name,
        columns.iter().map(|c| format!("\"{}\"", c)).collect::<Vec<_>>().join(", "),
    )
}

pub fn drop_index(index_name: &str) -> String {
    format!("DROP INDEX \"{}\"", index_name)
}

/// Map a canonical type name to its SQL representation.
fn map_type_to_sql(canonical: &str) -> &str {
    // Canonical names are already valid SQL types
    canonical
}
```

- [ ] **Step 3: Create main diff algorithm**

```rust
// engine/crates/garance-engine/src/diff/diff.rs

use std::collections::HashSet;
use crate::schema::json_schema::GaranceSchema;
use crate::schema::types::Schema;
use super::normalize::{normalize_type, normalize_default, pg_type_to_canonical};
use super::sql_gen;

/// Summary of changes detected by the diff.
#[derive(Debug, Default, serde::Serialize)]
pub struct DiffSummary {
    pub tables_created: usize,
    pub tables_dropped: usize,
    pub columns_added: usize,
    pub columns_dropped: usize,
    pub columns_modified: usize,
    pub indexes_created: usize,
    pub indexes_dropped: usize,
    pub foreign_keys_added: usize,
    pub foreign_keys_dropped: usize,
}

/// Result of a schema diff.
#[derive(Debug, serde::Serialize)]
pub struct DiffResult {
    pub statements: Vec<String>,
    pub summary: DiffSummary,
    pub has_destructive: bool,
}

/// Compare desired schema (from JSON) with current schema (from PG introspection).
/// Returns ordered SQL statements to transform current into desired.
pub fn diff(desired: &GaranceSchema, current: &Schema) -> DiffResult {
    let mut statements: DiffStatements = DiffStatements::default();
    let mut summary = DiffSummary::default();
    let mut has_destructive = false;

    let desired_tables: HashSet<&str> = desired.tables.keys().map(|s| s.as_str()).collect();
    let current_tables: HashSet<&str> = current.tables.keys().map(|s| s.as_str()).collect();

    // Phase 1: New tables
    for table_name in desired_tables.difference(&current_tables) {
        let table = &desired.tables[*table_name];
        let columns: Vec<_> = table.columns.iter().map(|(name, col)| {
            let normalized_type = normalize_type(&col.col_type);
            (
                name.clone(),
                normalized_type,
                col.nullable,
                col.default.clone(),
                col.primary_key,
                col.unique,
            )
        }).collect();
        statements.create_tables.push(sql_gen::create_table(table_name, &columns));
        summary.tables_created += 1;

        // Unique constraints on new tables (non-PK)
        for (name, col) in &table.columns {
            if col.unique && !col.primary_key {
                let constraint = format!("{}_{}_key", table_name, name);
                statements.add_unique.push(format!("ALTER TABLE \"{}\" ADD CONSTRAINT \"{}\" UNIQUE (\"{}\")", table_name, constraint, name));
            }
        }
    }

    // Phase 2: Dropped tables
    for table_name in current_tables.difference(&desired_tables) {
        // Drop FK first, then the table
        let current_table = &current.tables[*table_name];
        for fk in &current_table.foreign_keys {
            statements.drop_fks.push(sql_gen::drop_constraint(table_name, &fk.constraint_name));
            summary.foreign_keys_dropped += 1;
        }
        // Drop indexes
        for idx in &current_table.indexes {
            statements.drop_indexes.push(sql_gen::drop_index(&idx.name));
            summary.indexes_dropped += 1;
        }
        statements.drop_tables.push(sql_gen::drop_table(table_name));
        summary.tables_dropped += 1;
        has_destructive = true;
    }

    // Phase 3: Modified tables (exist in both)
    for table_name in desired_tables.intersection(&current_tables) {
        let desired_table = &desired.tables[*table_name];
        let current_table = &current.tables[*table_name];

        let desired_cols: HashSet<&str> = desired_table.columns.keys().map(|s| s.as_str()).collect();
        let current_cols: HashSet<&str> = current_table.columns.iter().map(|c| c.name.as_str()).collect();

        // New columns
        for col_name in desired_cols.difference(&current_cols) {
            let col = &desired_table.columns[*col_name];
            let normalized_type = normalize_type(&col.col_type);
            statements.add_columns.push(sql_gen::add_column(
                table_name, col_name, &normalized_type, col.nullable, col.default.as_deref(),
            ));
            summary.columns_added += 1;
        }

        // Dropped columns
        for col_name in current_cols.difference(&desired_cols) {
            statements.drop_columns.push(sql_gen::drop_column(table_name, col_name));
            summary.columns_dropped += 1;
            has_destructive = true;
        }

        // Modified columns (exist in both)
        for col_name in desired_cols.intersection(&current_cols) {
            let desired_col = &desired_table.columns[*col_name];
            let current_col = current_table.columns.iter().find(|c| c.name == *col_name).unwrap();

            let desired_type = normalize_type(&desired_col.col_type);
            let current_type = pg_type_to_canonical(&current_col.data_type);

            // Type change
            if desired_type != current_type {
                // Drop NOT NULL first if needed
                if !current_col.is_nullable {
                    statements.drop_not_null.push(sql_gen::drop_not_null(table_name, col_name));
                }
                statements.alter_types.push(sql_gen::alter_column_type(table_name, col_name, &desired_type));
                // Re-add NOT NULL after type change if desired
                if !desired_col.nullable {
                    statements.set_not_null.push(sql_gen::set_not_null(table_name, col_name));
                }
                summary.columns_modified += 1;
            } else {
                // Nullable change (only if type didn't change — type change handles it above)
                if desired_col.nullable && !current_col.is_nullable {
                    statements.drop_not_null.push(sql_gen::drop_not_null(table_name, col_name));
                    summary.columns_modified += 1;
                } else if !desired_col.nullable && current_col.is_nullable {
                    statements.set_not_null.push(sql_gen::set_not_null(table_name, col_name));
                    summary.columns_modified += 1;
                }
            }

            // Default change
            let desired_default = desired_col.default.as_ref().map(|d| normalize_default(d));
            let current_default = current_col.default_value.as_ref().map(|d| normalize_default(d));

            if desired_default != current_default {
                match &desired_col.default {
                    Some(def) => statements.set_defaults.push(sql_gen::set_default(table_name, col_name, def)),
                    None => statements.drop_defaults.push(sql_gen::drop_default(table_name, col_name)),
                }
                summary.columns_modified += 1;
            }
        }

        // Foreign keys diff
        diff_foreign_keys(table_name, desired_table, current_table, &mut statements, &mut summary);

        // Index diff
        diff_indexes(table_name, desired_table, current_table, &mut statements, &mut summary);
    }

    // FK for new tables (deferred)
    for table_name in desired_tables.difference(&current_tables) {
        let desired_table = &desired.tables[*table_name];
        add_fks_from_relations(table_name, desired_table, &mut statements, &mut summary);
    }

    // Assemble in correct order
    let mut all_statements = vec![];
    all_statements.extend(statements.create_tables);
    all_statements.extend(statements.add_columns);
    all_statements.extend(statements.drop_not_null);
    all_statements.extend(statements.alter_types);
    all_statements.extend(statements.set_defaults);
    all_statements.extend(statements.drop_defaults);
    all_statements.extend(statements.set_not_null);
    all_statements.extend(statements.add_fks);
    all_statements.extend(statements.add_unique);
    all_statements.extend(statements.create_indexes);
    all_statements.extend(statements.drop_indexes);
    all_statements.extend(statements.drop_unique);
    all_statements.extend(statements.drop_fks);
    all_statements.extend(statements.drop_columns);
    all_statements.extend(statements.drop_tables);

    DiffResult { statements: all_statements, summary, has_destructive }
}

/// Buckets for ordered SQL statements.
#[derive(Default)]
struct DiffStatements {
    create_tables: Vec<String>,
    add_columns: Vec<String>,
    drop_not_null: Vec<String>,
    alter_types: Vec<String>,
    set_defaults: Vec<String>,
    drop_defaults: Vec<String>,
    set_not_null: Vec<String>,
    add_fks: Vec<String>,
    add_unique: Vec<String>,
    create_indexes: Vec<String>,
    drop_indexes: Vec<String>,
    drop_unique: Vec<String>,
    drop_fks: Vec<String>,
    drop_columns: Vec<String>,
    drop_tables: Vec<String>,
}

fn diff_foreign_keys(
    table_name: &str,
    desired_table: &crate::schema::json_schema::GaranceTable,
    current_table: &crate::schema::types::Table,
    statements: &mut DiffStatements,
    summary: &mut DiffSummary,
) {
    // Desired FKs come from relations in GaranceTable
    for (rel_name, rel) in &desired_table.relations {
        // Check if this FK already exists in current
        let exists = current_table.foreign_keys.iter().any(|fk| {
            fk.referenced_table == rel.table && fk.columns.contains(&rel.foreign_key)
        });
        if !exists {
            let constraint_name = format!("fk_{}_{}", table_name, rel_name);
            statements.add_fks.push(sql_gen::add_foreign_key(
                table_name, &constraint_name, &[rel.foreign_key.clone()], &rel.table, &["id".to_string()],
            ));
            summary.foreign_keys_added += 1;
        }
    }

    // Drop FKs that are in current but not in desired
    for fk in &current_table.foreign_keys {
        let still_desired = desired_table.relations.values().any(|rel| {
            rel.table == fk.referenced_table && fk.columns.contains(&rel.foreign_key)
        });
        if !still_desired {
            statements.drop_fks.push(sql_gen::drop_constraint(table_name, &fk.constraint_name));
            summary.foreign_keys_dropped += 1;
        }
    }
}

fn diff_indexes(
    _table_name: &str,
    _desired_table: &crate::schema::json_schema::GaranceTable,
    _current_table: &crate::schema::types::Table,
    _statements: &mut DiffStatements,
    _summary: &mut DiffSummary,
) {
    // Index diff from GaranceSchema is limited in MVP.
    // Indexes are implicitly created for unique columns and FKs.
    // Explicit index management would require an index definition in the schema DSL.
    // For now: no-op. Indexes created by PG (for PK, unique, FK) are managed automatically.
}

fn add_fks_from_relations(
    table_name: &str,
    desired_table: &crate::schema::json_schema::GaranceTable,
    statements: &mut DiffStatements,
    summary: &mut DiffSummary,
) {
    for (rel_name, rel) in &desired_table.relations {
        let constraint_name = format!("fk_{}_{}", table_name, rel_name);
        statements.add_fks.push(sql_gen::add_foreign_key(
            table_name, &constraint_name, &[rel.foreign_key.clone()], &rel.table, &["id".to_string()],
        ));
        summary.foreign_keys_added += 1;
    }
}
```

- [ ] **Step 4: Create diff module root**

```rust
// engine/crates/garance-engine/src/diff/mod.rs
pub mod normalize;
pub mod sql_gen;
pub mod diff;

pub use diff::{diff, DiffResult, DiffSummary};
```

- [ ] **Step 5: Add module to lib.rs**

Add `pub mod diff;` to `engine/crates/garance-engine/src/lib.rs`.

- [ ] **Step 6: Add sha2 dependency**

Add to `engine/Cargo.toml` workspace deps: `sha2 = "0.10"`
Add to `engine/crates/garance-engine/Cargo.toml`: `sha2.workspace = true`

- [ ] **Step 7: Write diff integration tests**

Create `engine/crates/garance-engine/tests/diff_test.rs` with all 13 diff tests from the spec. Each test builds a `GaranceSchema` (desired) and a `Schema` (current) and asserts the produced SQL.

Tests to include:
- `test_diff_create_table` — new table → CREATE TABLE
- `test_diff_drop_table` — removed table → DROP TABLE
- `test_diff_add_column` — new column → ALTER TABLE ADD COLUMN
- `test_diff_drop_column` — removed column → ALTER TABLE DROP COLUMN
- `test_diff_change_type` — type change → ALTER COLUMN TYPE
- `test_diff_change_nullable` — nullable change → SET/DROP NOT NULL
- `test_diff_change_default` — default change → SET/DROP DEFAULT
- `test_diff_add_foreign_key` — new FK → ADD CONSTRAINT
- `test_diff_drop_foreign_key` — removed FK → DROP CONSTRAINT
- `test_diff_add_unique` — unique column → ADD CONSTRAINT UNIQUE
- `test_diff_no_changes` — identical → empty statements
- `test_diff_statement_ordering` — FKs after tables, drops in reverse order
- `test_diff_destructive_flag` — DROP TABLE → has_destructive true
- `test_diff_type_normalization` — varchar vs text → no false positive
- `test_diff_default_normalization` — now() vs CURRENT_TIMESTAMP → no false positive
- `test_diff_self_referential_fk` — FK on same table → deferred to ALTER TABLE

The tests construct `GaranceSchema` and `Schema` structs directly in code (no database needed — pure unit tests).

- [ ] **Step 8: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/engine && cargo test -p garance-engine`
Expected: All tests pass.

- [ ] **Step 9: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add engine/
git commit -m ":sparkles: feat(engine): add diff engine for schema migration generation"
```

---

## Task 3: Engine API — Migrate Preview & Apply Endpoints

**Files:**
- Modify: `engine/crates/garance-engine/src/api/routes.rs` — add `migrate_preview` and `migrate_apply` handlers
- Modify: `engine/crates/garance-engine/src/api/mod.rs` — register 2 new routes
- Create: `engine/crates/garance-engine/tests/migrate_test.rs` — 3 integration tests

- [ ] **Step 1: Write `migrate_preview` handler**

```rust
/// POST /api/v1/_migrate/preview — diff desired schema vs PG, return SQL
pub async fn migrate_preview(
    State(state): State<AppState>,
    Json(body): Json<serde_json::Value>,
) -> Result<impl IntoResponse, ApiError> {
    let desired: crate::schema::json_schema::GaranceSchema = serde_json::from_value(
        body.get("schema").cloned().unwrap_or(body.clone())
    ).map_err(|e| ApiError {
        error: super::error::ApiErrorBody {
            code: "VALIDATION_ERROR".into(), message: format!("invalid schema JSON: {}", e), status: 400, details: None,
        },
    })?;

    // Introspect current PG state
    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
    })?;
    let current = crate::schema::introspect(&client, "public").await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
    })?;

    let diff_result = crate::diff::diff(&desired, &current);

    Ok(Json(json!({
        "statements": diff_result.statements,
        "summary": diff_result.summary,
        "has_destructive": diff_result.has_destructive,
    })))
}
```

- [ ] **Step 2: Write `migrate_apply` handler**

```rust
use sha2::{Sha256, Digest};

#[derive(Deserialize)]
pub struct MigrateApplyRequest {
    sql: String,
    filename: String,
}

/// POST /api/v1/_migrate/apply — execute migration SQL, record in tracking table
pub async fn migrate_apply(
    State(state): State<AppState>,
    Json(body): Json<MigrateApplyRequest>,
) -> Result<impl IntoResponse, ApiError> {
    let client = state.pool.get().await.map_err(|e| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
    })?;

    let pg_err = |e: tokio_postgres::Error| ApiError {
        error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
    };

    // Ensure tracking table exists
    client.execute(
        "CREATE SCHEMA IF NOT EXISTS garance_platform", &[]
    ).await.map_err(&pg_err)?;
    client.execute(
        "CREATE TABLE IF NOT EXISTS garance_platform.migrations (
            id SERIAL PRIMARY KEY,
            filename TEXT UNIQUE NOT NULL,
            checksum TEXT NOT NULL,
            applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )", &[]
    ).await.map_err(&pg_err)?;

    // Check if already applied
    let existing = client.query_opt(
        "SELECT filename FROM garance_platform.migrations WHERE filename = $1",
        &[&body.filename],
    ).await.map_err(&pg_err)?;

    if existing.is_some() {
        return Err(ApiError {
            error: super::error::ApiErrorBody {
                code: "CONFLICT".into(), message: format!("migration '{}' already applied", body.filename), status: 409, details: None,
            },
        });
    }

    // Compute checksum
    let mut hasher = Sha256::new();
    hasher.update(body.sql.as_bytes());
    let checksum = format!("{:x}", hasher.finalize());

    // Execute in transaction (use tokio-postgres Transaction API, not manual BEGIN)
    let tx = client.build_transaction().start().await.map_err(&pg_err)?;

    match tx.batch_execute(&body.sql).await {
        Ok(_) => {
            // Record in tracking
            tx.execute(
                "INSERT INTO garance_platform.migrations (filename, checksum) VALUES ($1, $2)",
                &[&body.filename, &checksum],
            ).await.map_err(&pg_err)?;

            tx.commit().await.map_err(&pg_err)?;

            // Reload schema (need a fresh connection since tx consumed the previous one)
            let reload_client = state.pool.get().await.map_err(|e| ApiError {
                error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
            })?;
            let new_schema = crate::schema::introspect(&reload_client, "public").await.map_err(|e| ApiError {
                error: super::error::ApiErrorBody { code: "INTERNAL_ERROR".into(), message: e.to_string(), status: 500, details: None },
            })?;
            let table_count = new_schema.tables.len();
            let mut schema = state.schema.write().await;
            *schema = new_schema;

            Ok((StatusCode::CREATED, Json(json!({
                "applied": true,
                "filename": body.filename,
                "tables_after": table_count,
            }))))
        }
        Err(e) => {
            // Transaction is automatically rolled back when dropped
            Err(ApiError {
                error: super::error::ApiErrorBody {
                    code: "VALIDATION_ERROR".into(), message: format!("migration failed: {}", e), status: 400, details: None,
                },
            })
        }
    }
}
```

- [ ] **Step 3: Register routes**

Add to `api/mod.rs`:
```rust
.route("/api/v1/_migrate/preview", post(routes::migrate_preview))
.route("/api/v1/_migrate/apply", post(routes::migrate_apply))
```

- [ ] **Step 4: Write integration tests**

Create `engine/crates/garance-engine/tests/migrate_test.rs` with:
- `test_migrate_preview_endpoint` — sends a schema JSON, gets SQL back
- `test_migrate_apply_endpoint` — applies SQL, checks tracking table
- `test_migrate_tracking` — same filename returns 409
- `test_migrate_apply_rollback_on_error` — invalid SQL → rollback, nothing recorded

- [ ] **Step 5: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/engine && cargo test -p garance-engine`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add engine/
git commit -m ":sparkles: feat(engine): add _migrate/preview and _migrate/apply endpoints"
```

---

## Task 4: @garance/schema — Add `renamedFrom`

**Files:**
- Modify: `packages/schema/src/column.ts` — add `renamedFrom` method + `renamed_from` in output
- Modify: `packages/schema/src/types.ts` — add `renamed_from` to GaranceColumn
- Modify: `packages/schema/src/__tests__/builders.test.ts` — add test
- Modify: `cli/internal/project/templates.go` — add package.json to init template

- [ ] **Step 1: Add `renamedFrom` to ColumnBuilder**

In `packages/schema/src/column.ts`, add to the `ColumnBuilder` class:

```typescript
private _renamedFrom?: string

renamedFrom(oldName: string): this {
  this._renamedFrom = oldName
  return this
}
```

Update `_build()` to include it:
```typescript
...(this._renamedFrom && { renamed_from: this._renamedFrom }),
```

- [ ] **Step 2: Add `renamed_from` to types**

In `packages/schema/src/types.ts`, add to `GaranceColumn`:
```typescript
renamed_from?: string
```

- [ ] **Step 3: Add test**

```typescript
it('supports renamedFrom hint', () => {
  const col = column.text().notNull().renamedFrom('old_name')
  const built = col._build()
  expect(built.renamed_from).toBe('old_name')
})
```

- [ ] **Step 4: Update CLI init template**

In `cli/internal/project/templates.go`, update `DefaultSchemaTemplate()` to also create a `package.json` template, and update `project.go` `Init()` to write it.

Add to templates.go:
```go
func DefaultPackageJSONTemplate(name string) string {
	return fmt.Sprintf(`{
  "private": true,
  "dependencies": {
    "@garance/schema": "^0.1.0"
  }
}
`, name)
}
```

Update `project.go` `Init()` to write `package.json` if it doesn't already exist.

- [ ] **Step 5: Run tests**

```bash
cd /Users/jh3ady/Development/Projects/garance/packages/schema && npm test
cd /Users/jh3ady/Development/Projects/garance/cli && go test ./...
```

- [ ] **Step 6: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add packages/schema/ cli/
git commit -m ":sparkles: feat(schema): add renamedFrom hint and package.json template in CLI init"
```

---

## Task 5: CLI — Wire `garance db migrate` to Pipeline

**Files:**
- Modify: `cli/cmd/db.go` — rewrite `dbMigrateCmd` to use the pipeline
- Create: `cli/internal/schema/compile.go` — TS compilation helper

- [ ] **Step 1: Write schema compiler**

```go
// cli/internal/schema/compile.go
package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Compile executes garance.schema.ts and returns the compiled JSON.
func Compile(dir string) (json.RawMessage, error) {
	schemaFile := filepath.Join(dir, "garance.schema.ts")
	if _, err := os.Stat(schemaFile); err != nil {
		return nil, fmt.Errorf("garance.schema.ts not found in %s", dir)
	}

	// Write temp compilation script
	tmpScript := filepath.Join(dir, ".garance-compile.mjs")
	script := `
import { compile } from '@garance/schema';
const mod = await import('./garance.schema.ts');
const schema = mod.default;
const result = compile(schema);
console.log(JSON.stringify(result));
`
	if err := os.WriteFile(tmpScript, []byte(script), 0644); err != nil {
		return nil, fmt.Errorf("failed to write compile script: %w", err)
	}
	defer os.Remove(tmpScript)

	// Execute via npx tsx
	cmd := exec.Command("npx", "tsx", tmpScript)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("schema compilation failed: %w\nMake sure Node.js is installed and run 'npm install' in your project", err)
	}

	// Validate it's valid JSON
	var raw json.RawMessage
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("schema compilation produced invalid JSON: %w", err)
	}

	// Write garance.schema.json
	if err := os.WriteFile(filepath.Join(dir, "garance.schema.json"), output, 0644); err != nil {
		return nil, fmt.Errorf("failed to write garance.schema.json: %w", err)
	}

	return raw, nil
}
```

- [ ] **Step 2: Rewrite `garance db migrate`**

```go
// In cli/cmd/db.go — replace the existing dbMigrateCmd RunE

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Generate and apply migrations from garance.schema.ts",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		yes, _ := cmd.Flags().GetBool("yes")
		engineURL := os.Getenv("ENGINE_URL")
		if engineURL == "" {
			engineURL = "http://localhost:4000"
		}

		// Step 1: Compile schema
		fmt.Println("Compiling schema...")
		schemaJSON, err := schema.Compile(dir)
		if err != nil {
			return err
		}
		fmt.Println("✓ garance.schema.ts → garance.schema.json")

		// Step 2: Preview
		fmt.Println("\nGenerating migration preview...")
		previewBody, _ := json.Marshal(map[string]json.RawMessage{"schema": schemaJSON})
		resp, err := http.Post(engineURL+"/api/v1/_migrate/preview", "application/json", bytes.NewReader(previewBody))
		if err != nil {
			return fmt.Errorf("failed to connect to Engine at %s: %w", engineURL, err)
		}
		defer resp.Body.Close()

		var preview struct {
			Statements     []string `json:"statements"`
			Summary        map[string]int `json:"summary"`
			HasDestructive bool `json:"has_destructive"`
		}
		json.NewDecoder(resp.Body).Decode(&preview)

		if len(preview.Statements) == 0 {
			fmt.Println("✓ Schema is up to date — no changes needed")
			return nil
		}

		// Display preview
		fmt.Printf("✓ %d changes detected:\n", len(preview.Statements))
		for _, stmt := range preview.Statements {
			// Show first line of each statement
			line := strings.SplitN(stmt, "\n", 2)[0]
			if strings.HasPrefix(strings.ToUpper(line), "DROP") {
				fmt.Printf("  \033[31m- %s\033[0m\n", line) // red for destructive
			} else {
				fmt.Printf("  \033[32m+ %s\033[0m\n", line) // green for additive
			}
		}

		// Confirm if destructive
		if preview.HasDestructive && !yes {
			fmt.Print("\n⚠ This migration contains destructive changes (data loss). Continue? [y/N] ")
			var answer string
			fmt.Scanln(&answer)
			if answer != "y" && answer != "Y" {
				fmt.Println("Migration cancelled.")
				return nil
			}
		}

		// Step 3: Save migration file
		sql := strings.Join(preview.Statements, ";\n\n") + ";"
		timestamp := time.Now().Format("20060102150405")
		description := generateDescription(preview.Summary)
		filename := fmt.Sprintf("%s_%s.sql", timestamp, description)

		migrationsDir := filepath.Join(dir, "migrations")
		os.MkdirAll(migrationsDir, 0755)
		migrationPath := filepath.Join(migrationsDir, filename)
		os.WriteFile(migrationPath, []byte(sql), 0644)
		fmt.Printf("\nMigration saved: migrations/%s\n", filename)

		// Step 4: Apply
		fmt.Println("\nApplying migration...")
		applyBody, _ := json.Marshal(map[string]string{"sql": sql, "filename": filename})
		resp2, err := http.Post(engineURL+"/api/v1/_migrate/apply", "application/json", bytes.NewReader(applyBody))
		if err != nil {
			return fmt.Errorf("failed to apply migration: %w", err)
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != 201 {
			var errResp map[string]interface{}
			json.NewDecoder(resp2.Body).Decode(&errResp)
			return fmt.Errorf("migration failed: %v", errResp)
		}

		var applied struct {
			TablesAfter int `json:"tables_after"`
		}
		json.NewDecoder(resp2.Body).Decode(&applied)
		fmt.Printf("✓ Migration applied (%d tables total)\n", applied.TablesAfter)

		return nil
	},
}

func generateDescription(summary map[string]int) string {
	parts := []string{}
	if n := summary["tables_created"]; n > 0 {
		parts = append(parts, fmt.Sprintf("create_%d_tables", n))
	}
	if n := summary["columns_added"]; n > 0 {
		parts = append(parts, fmt.Sprintf("add_%d_columns", n))
	}
	if n := summary["tables_dropped"]; n > 0 {
		parts = append(parts, fmt.Sprintf("drop_%d_tables", n))
	}
	if n := summary["columns_dropped"]; n > 0 {
		parts = append(parts, fmt.Sprintf("drop_%d_columns", n))
	}
	if len(parts) == 0 {
		return "schema_update"
	}
	return strings.Join(parts, "_and_")
}
```

Add `--yes` flag:
```go
func init() {
	dbMigrateCmd.Flags().Bool("yes", false, "Skip confirmation for destructive migrations")
	// ... existing
}
```

Add imports: `"bytes"`, `"encoding/json"`, `"net/http"`, `"strings"`, `"time"`, `"path/filepath"`, and the schema package.

- [ ] **Step 3: Update Gateway with migrate proxy routes**

Add to `services/gateway/internal/proxy/engine.go`:

```go
mux.HandleFunc("POST /api/v1/_migrate/preview", p.MigratePreview)
mux.HandleFunc("POST /api/v1/_migrate/apply", p.MigrateApply)
```

And simple pass-through handlers that forward to Engine gRPC (or for MVP, HTTP pass-through since the proto update can be deferred):

For MVP, add HTTP reverse proxy handlers that forward to the Engine HTTP directly.

- [ ] **Step 4: Verify build**

```bash
cd /Users/jh3ady/Development/Projects/garance/cli && go build -o garance .
./garance db migrate --help
```

- [ ] **Step 5: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add cli/ services/gateway/
git commit -m ":sparkles: feat(cli): wire garance db migrate to schema pipeline"
```

---

## Summary

| Task | Description | Tests |
|---|---|---|
| 1 | Fix ForeignKey model (add constraint_name) | 1 updated |
| 2 | Diff engine (normalize, sql_gen, diff algorithm) | ~17 (14 diff + 2 normalize unit + 1 unique) |
| 3 | Engine API (migrate/preview, migrate/apply) | 4 |
| 4 | @garance/schema (renamedFrom) + CLI init template | 1 |
| 5 | CLI pipeline (compile → preview → save → apply) | 0 (build verification) |
| **Total** | | **~23** |

### Not in this plan (deferred)

- gRPC proto update for MigratePreview/MigrateApply (Gateway currently proxies HTTP→HTTP for these routes)
- Rename detection (interactive, Prisma-style)
- Migration rollback
- Schema validation (referential integrity checks before generating SQL)
- Explicit index management in the schema DSL

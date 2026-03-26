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

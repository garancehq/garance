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

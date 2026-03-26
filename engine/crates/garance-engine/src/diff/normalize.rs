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

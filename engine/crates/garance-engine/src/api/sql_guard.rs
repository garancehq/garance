const BLOCKED_SCHEMAS: &[&str] = &[
    "garance_auth.",
    "garance_storage.",
    "garance_platform.",
    "garance_audit.",
    "information_schema.",
];

const MAX_SQL_SIZE: usize = 64 * 1024; // 64KB

#[derive(Debug)]
pub enum SqlValidationError {
    Empty,
    TooLarge(usize),
    MultiStatement,
    BlockedSchema(String),
}

impl std::fmt::Display for SqlValidationError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            SqlValidationError::Empty => write!(f, "SQL query is empty"),
            SqlValidationError::TooLarge(size) => write!(f, "SQL query exceeds maximum size ({}KB > 64KB)", size / 1024),
            SqlValidationError::MultiStatement => write!(f, "multi-statement SQL is not allowed (remove semicolons)"),
            SqlValidationError::BlockedSchema(schema) => write!(f, "references to internal schema '{}' are not allowed", schema),
        }
    }
}

/// Validate SQL before execution. Returns Ok(()) if the SQL is safe to execute.
pub fn validate_sql(sql: &str) -> Result<(), SqlValidationError> {
    let trimmed = sql.trim();

    // Empty check
    if trimmed.is_empty() {
        return Err(SqlValidationError::Empty);
    }

    // Size check
    if sql.len() > MAX_SQL_SIZE {
        return Err(SqlValidationError::TooLarge(sql.len()));
    }

    // Multi-statement check: reject if SQL contains ';' (except at the very end)
    let without_trailing = trimmed.trim_end_matches(';').trim();
    if without_trailing.contains(';') {
        return Err(SqlValidationError::MultiStatement);
    }

    // Blocked schema check (case-insensitive)
    let lower = sql.to_lowercase();
    for schema in BLOCKED_SCHEMAS {
        if lower.contains(schema) {
            let display_name = schema.trim_end_matches('.');
            return Err(SqlValidationError::BlockedSchema(display_name.to_string()));
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_valid_sql() {
        assert!(validate_sql("SELECT * FROM users").is_ok());
        assert!(validate_sql("SELECT * FROM users;").is_ok()); // trailing semicolon OK
        assert!(validate_sql("INSERT INTO users (name) VALUES ('test')").is_ok());
    }

    #[test]
    fn test_empty_sql() {
        assert!(matches!(validate_sql(""), Err(SqlValidationError::Empty)));
        assert!(matches!(validate_sql("   "), Err(SqlValidationError::Empty)));
    }

    #[test]
    fn test_multi_statement() {
        assert!(matches!(
            validate_sql("SELECT 1; DROP TABLE users"),
            Err(SqlValidationError::MultiStatement)
        ));
        assert!(matches!(
            validate_sql("SELECT 1; SELECT 2;"),
            Err(SqlValidationError::MultiStatement)
        ));
    }

    #[test]
    fn test_blocked_schema() {
        assert!(matches!(
            validate_sql("SELECT * FROM garance_auth.users"),
            Err(SqlValidationError::BlockedSchema(_))
        ));
        assert!(matches!(
            validate_sql("SELECT * FROM GARANCE_AUTH.users"),
            Err(SqlValidationError::BlockedSchema(_))
        ));
        assert!(matches!(
            validate_sql("SELECT * FROM garance_storage.files"),
            Err(SqlValidationError::BlockedSchema(_))
        ));
    }

    #[test]
    fn test_size_limit() {
        let huge = "SELECT ".to_string() + &"x".repeat(MAX_SQL_SIZE);
        assert!(matches!(validate_sql(&huge), Err(SqlValidationError::TooLarge(_))));
    }
}

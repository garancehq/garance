use crate::schema::json_schema::{GaranceAccess, AccessRule, AccessCondition};

/// Generate SQL to enable RLS on a table (always, regardless of access rules).
pub fn enable_rls(table_name: &str) -> Vec<String> {
    vec![
        format!("ALTER TABLE \"{}\" ENABLE ROW LEVEL SECURITY", table_name),
        format!("ALTER TABLE \"{}\" FORCE ROW LEVEL SECURITY", table_name),
    ]
}

/// Generate SQL to disable RLS on a table.
pub fn disable_rls(table_name: &str) -> Vec<String> {
    vec![
        format!("ALTER TABLE \"{}\" DISABLE ROW LEVEL SECURITY", table_name),
    ]
}

/// Generate all RLS policy statements for a table from its access rules.
/// Returns (policy_name, sql) pairs.
pub fn generate_policies(table_name: &str, access: &GaranceAccess) -> Vec<(String, String)> {
    let mut policies = vec![];

    // SELECT policy from read
    if let Some(ref read) = access.read {
        let using_clause = access_rule_to_sql(read);
        let name = format!("garance_select_{}", table_name);
        let sql = format!(
            "CREATE POLICY \"{}\" ON \"{}\" FOR SELECT USING ({})",
            name, table_name, using_clause
        );
        policies.push((name, sql));
    }

    // INSERT policy from write
    if let Some(ref write) = access.write {
        let check_clause = access_rule_to_sql(write);
        let name = format!("garance_insert_{}", table_name);
        let sql = format!(
            "CREATE POLICY \"{}\" ON \"{}\" FOR INSERT WITH CHECK ({})",
            name, table_name, check_clause
        );
        policies.push((name, sql));

        // UPDATE policy from write (USING + WITH CHECK)
        let name = format!("garance_update_{}", table_name);
        let sql = format!(
            "CREATE POLICY \"{}\" ON \"{}\" FOR UPDATE USING ({}) WITH CHECK ({})",
            name, table_name, check_clause, check_clause
        );
        policies.push((name, sql));
    }

    // DELETE policy — explicit only, does NOT inherit from write
    if let Some(ref delete) = access.delete {
        let using_clause = access_rule_to_sql(delete);
        let name = format!("garance_delete_{}", table_name);
        let sql = format!(
            "CREATE POLICY \"{}\" ON \"{}\" FOR DELETE USING ({})",
            name, table_name, using_clause
        );
        policies.push((name, sql));
    }

    policies
}

/// Drop all garance policies on a table.
pub fn drop_policies(table_name: &str, policy_names: &[String]) -> Vec<String> {
    policy_names.iter().map(|name| {
        format!("DROP POLICY IF EXISTS \"{}\" ON \"{}\"", name, table_name)
    }).collect()
}

/// Convert an AccessRule to a SQL expression for USING/WITH CHECK.
pub fn access_rule_to_sql(rule: &AccessRule) -> String {
    match rule {
        AccessRule::Public(s) => match s.as_str() {
            "public" => "true".to_string(),
            "authenticated" => "current_setting('request.user_id', true) IS NOT NULL".to_string(),
            other => format!("false /* unknown rule: {} */", other),
        },
        AccessRule::Conditions(conditions) => {
            if conditions.is_empty() {
                return "false".to_string();
            }
            let parts: Vec<String> = conditions.iter().map(condition_to_sql).collect();
            if parts.len() == 1 {
                parts[0].clone()
            } else {
                // Multiple conditions combined with OR
                parts.iter().map(|p| format!("({})", p)).collect::<Vec<_>>().join(" OR ")
            }
        }
    }
}

/// Convert a single AccessCondition to a SQL expression.
pub fn condition_to_sql(cond: &AccessCondition) -> String {
    match cond.condition_type.as_str() {
        "isOwner" => {
            let column = cond.column.as_deref().unwrap_or("user_id");
            format!(
                "\"{}\"::text = current_setting('request.user_id', true)",
                column
            )
        }
        "isAuthenticated" => {
            "current_setting('request.user_id', true) IS NOT NULL".to_string()
        }
        "where" => {
            match &cond.filters {
                Some(filters) => {
                    let conditions: Vec<String> = filters.iter().map(|(col, val)| {
                        let sql_val = value_to_sql_literal(val);
                        format!("\"{}\" = {}", col, sql_val)
                    }).collect();
                    conditions.join(" AND ")
                }
                None => "true".to_string(),
            }
        }
        other => format!("false /* unknown condition: {} */", other),
    }
}

/// Convert a JSON value to a safe SQL literal (equivalent of quote_literal).
pub fn value_to_sql_literal(val: &serde_json::Value) -> String {
    match val {
        serde_json::Value::Bool(b) => b.to_string(),
        serde_json::Value::Number(n) => n.to_string(),
        serde_json::Value::String(s) => {
            // Escape single quotes by doubling them (PostgreSQL standard)
            let escaped = s.replace('\'', "''");
            format!("'{}'", escaped)
        }
        serde_json::Value::Null => "NULL".to_string(),
        _ => "NULL".to_string(),
    }
}

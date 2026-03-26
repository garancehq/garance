use tokio_postgres::Client;
use tracing::info;

const CREATE_NOTIFY_FUNCTION: &str = r#"
CREATE OR REPLACE FUNCTION garance_notify_change() RETURNS trigger AS $$
DECLARE
  payload json;
  payload_text text;
BEGIN
  payload = json_build_object(
    'table', TG_TABLE_NAME,
    'schema', TG_TABLE_SCHEMA,
    'event', TG_OP,
    'new', CASE WHEN TG_OP IN ('INSERT', 'UPDATE') THEN row_to_json(NEW) ELSE NULL END,
    'old', CASE WHEN TG_OP IN ('UPDATE', 'DELETE') THEN row_to_json(OLD) ELSE NULL END,
    'timestamp', now()
  );
  payload_text = payload::text;
  IF length(payload_text) > 7168 THEN
    payload = json_build_object(
      'table', TG_TABLE_NAME,
      'schema', TG_TABLE_SCHEMA,
      'event', TG_OP,
      'new', NULL,
      'old', NULL,
      'timestamp', now(),
      'truncated', true
    );
    payload_text = payload::text;
  END IF;
  PERFORM pg_notify('garance_changes', payload_text);
  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
"#;

/// Create the garance_notify_change() function if it doesn't exist.
pub async fn ensure_notify_function(client: &Client) -> Result<(), tokio_postgres::Error> {
    client.batch_execute(CREATE_NOTIFY_FUNCTION).await?;
    info!("realtime notify function created/updated");
    Ok(())
}

/// Attach the realtime trigger to all user tables that don't have it yet.
pub async fn attach_triggers(client: &Client, schema_name: &str) -> Result<usize, tokio_postgres::Error> {
    let tables = client.query(
        "SELECT table_name FROM information_schema.tables
         WHERE table_schema = $1 AND table_type = 'BASE TABLE'",
        &[&schema_name],
    ).await?;

    let mut attached = 0;
    for row in &tables {
        let table_name: &str = row.get("table_name");

        // Check if trigger already exists
        let exists = client.query_one(
            "SELECT EXISTS (
                SELECT 1 FROM pg_trigger
                WHERE tgname = 'garance_realtime_trigger'
                AND tgrelid = $1::regclass
            )",
            &[&format!("{}.{}", schema_name, table_name)],
        ).await?;

        let has_trigger: bool = exists.get(0);
        if !has_trigger {
            let sql = format!(
                "CREATE TRIGGER garance_realtime_trigger
                 AFTER INSERT OR UPDATE OR DELETE ON \"{}\".\"{}\"
                 FOR EACH ROW EXECUTE FUNCTION garance_notify_change()",
                schema_name, table_name
            );
            client.batch_execute(&sql).await?;
            attached += 1;
        }
    }

    if attached > 0 {
        info!(schema = schema_name, count = attached, "realtime triggers attached");
    }
    Ok(attached)
}

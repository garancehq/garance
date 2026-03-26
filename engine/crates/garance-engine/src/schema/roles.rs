use tokio_postgres::Client;
use tracing::info;

/// Create garance_anon and garance_authenticated roles (idempotent).
pub async fn ensure_roles(client: &Client) -> Result<(), tokio_postgres::Error> {
    client.batch_execute(r#"
        DO $$ BEGIN
            IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'garance_anon') THEN
                CREATE ROLE garance_anon NOLOGIN;
            END IF;
            IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'garance_authenticated') THEN
                CREATE ROLE garance_authenticated NOLOGIN;
            END IF;
        END $$;

        GRANT USAGE ON SCHEMA public TO garance_anon, garance_authenticated;
        GRANT SELECT ON ALL TABLES IN SCHEMA public TO garance_anon;
        GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO garance_authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO garance_anon;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO garance_authenticated;
    "#).await?;

    info!("PG roles garance_anon and garance_authenticated ensured");
    Ok(())
}

/// Grant permissions on a specific table to the roles.
/// Called after creating new tables (migrate/reload).
pub async fn grant_table_permissions(client: &Client, table_name: &str) -> Result<(), tokio_postgres::Error> {
    let sql = format!(
        "GRANT SELECT ON \"{}\" TO garance_anon; GRANT SELECT, INSERT, UPDATE, DELETE ON \"{}\" TO garance_authenticated;",
        table_name, table_name
    );
    client.batch_execute(&sql).await?;
    Ok(())
}

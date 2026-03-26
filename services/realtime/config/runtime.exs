import Config

if config_env() == :prod do
  database_url = System.get_env("DATABASE_URL") || raise "DATABASE_URL not set"
  secret_key_base = System.get_env("SECRET_KEY_BASE") || raise "SECRET_KEY_BASE not set"
  port = String.to_integer(System.get_env("PORT") || "4003")

  config :realtime, Realtime.PgListener,
    database_url: database_url

  config :realtime, RealtimeWeb.Endpoint,
    http: [port: port],
    secret_key_base: secret_key_base,
    server: true
end

import Config

config :realtime, RealtimeWeb.Endpoint,
  http: [port: 4003],
  debug_errors: true,
  code_reloader: false,
  check_origin: false,
  secret_key_base: "dev-secret-key-base-at-least-64-characters-long-for-phoenix-to-accept-it-ok"

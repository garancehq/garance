import Config

config :realtime, RealtimeWeb.Endpoint,
  http: [port: 4003],
  secret_key_base: "test-secret-key-base-at-least-64-characters-long-for-phoenix-to-accept-it-ok",
  server: false

config :logger, level: :warning

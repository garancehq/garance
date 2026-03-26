import Config

config :realtime, RealtimeWeb.Endpoint,
  url: [host: "localhost"],
  render_errors: [formats: [json: RealtimeWeb.ErrorJSON]],
  pubsub_server: Realtime.PubSub,
  live_view: [signing_salt: "realtime"]

config :logger, :console,
  format: "$time $metadata[$level] $message\n",
  metadata: [:request_id]

import_config "#{config_env()}.exs"

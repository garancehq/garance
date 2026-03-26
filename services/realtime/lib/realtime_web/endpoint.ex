defmodule RealtimeWeb.Endpoint do
  use Phoenix.Endpoint, otp_app: :realtime

  socket "/socket", RealtimeWeb.RealtimeSocket,
    websocket: [timeout: 45_000],
    longpoll: false

  plug Plug.Parsers,
    parsers: [:json],
    pass: ["application/json"],
    json_decoder: Jason

  plug RealtimeWeb.Router
end

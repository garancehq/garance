defmodule RealtimeWeb.HealthController do
  use Phoenix.Controller, formats: [:json]

  def index(conn, _params) do
    send_resp(conn, 200, "ok")
  end
end

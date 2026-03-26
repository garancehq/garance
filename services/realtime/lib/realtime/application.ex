defmodule Realtime.Application do
  use Application

  @impl true
  def start(_type, _args) do
    database_url = System.get_env("DATABASE_URL") || "postgresql://postgres:postgres@localhost:5432/garance"

    children = [
      {Phoenix.PubSub, name: Realtime.PubSub},
      {Realtime.PgListener, database_url: database_url},
      Realtime.SubscriptionRegistry,
      RealtimeWeb.Endpoint
    ]

    opts = [strategy: :one_for_one, name: Realtime.Supervisor]
    Supervisor.start_link(children, opts)
  end
end

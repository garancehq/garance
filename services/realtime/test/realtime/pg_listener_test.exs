defmodule Realtime.PgListenerTest do
  use ExUnit.Case

  test "broadcasts pg_change on NOTIFY" do
    # Subscribe to PubSub to receive changes
    Phoenix.PubSub.subscribe(Realtime.PubSub, "pg_changes")

    # Get a direct PG connection to send NOTIFY
    database_url = System.get_env("DATABASE_URL") || "postgresql://postgres:postgres@localhost:5432/garance"
    pg_opts = Realtime.PgListener.parse_database_url(database_url)
    {:ok, conn} = Postgrex.start_link(pg_opts)

    payload = Jason.encode!(%{
      "table" => "test_table",
      "schema" => "public",
      "event" => "INSERT",
      "new" => %{"id" => "abc-123", "name" => "test"},
      "old" => nil,
      "timestamp" => "2026-03-26T12:00:00Z"
    })

    Postgrex.query!(conn, "SELECT pg_notify('garance_changes', $1)", [payload])

    assert_receive {:pg_change, change}, 5000
    assert change["table"] == "test_table"
    assert change["event"] == "INSERT"
    assert change["new"]["id"] == "abc-123"

    GenServer.stop(conn)
  end
end

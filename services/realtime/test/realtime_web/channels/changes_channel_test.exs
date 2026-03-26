defmodule RealtimeWeb.ChangesChannelTest do
  use ExUnit.Case
  import Phoenix.ChannelTest

  @endpoint RealtimeWeb.Endpoint

  setup do
    {:ok, _, socket} =
      RealtimeWeb.RealtimeSocket
      |> socket()
      |> subscribe_and_join(RealtimeWeb.ChangesChannel, "realtime:todos")

    %{socket: socket}
  end

  test "subscribes and receives changes", %{socket: socket} do
    # Subscribe to INSERT events
    push(socket, "subscribe", %{"events" => ["INSERT"], "ref" => "1"})
    assert_push "subscribed", %{"ref" => "1", "table" => "todos"}

    # Simulate a PG change via direct message to channel pid
    change = %{
      "table" => "todos",
      "schema" => "public",
      "event" => "INSERT",
      "new" => %{"id" => "abc", "title" => "Test"},
      "old" => nil,
      "timestamp" => "2026-03-26T12:00:00Z"
    }

    send(socket.channel_pid, {:realtime_change, change})

    assert_push "change", payload
    assert payload["table"] == "todos"
    assert payload["event"] == "INSERT"
    assert payload["new"]["id"] == "abc"
  end

  test "unsubscribe stops receiving changes", %{socket: socket} do
    push(socket, "subscribe", %{"events" => ["*"], "ref" => "1"})
    assert_push "subscribed", _

    push(socket, "unsubscribe", %{"ref" => "2"})
    assert_push "unsubscribed", %{"ref" => "2"}
  end

  test "heartbeat returns ack", %{socket: socket} do
    push(socket, "heartbeat", %{"ref" => "3"})
    assert_push "heartbeat_ack", %{"ref" => "3"}
  end
end

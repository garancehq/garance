defmodule RealtimeWeb.ChangesChannel do
  use Phoenix.Channel
  require Logger

  @impl true
  def join("realtime:" <> table, params, socket) do
    user_id = Map.get(params, "user_id", nil)
    Logger.info("Client joined realtime:#{table}")
    {:ok, assign(socket, table: table, user_id: user_id)}
  end

  @impl true
  def handle_in("subscribe", payload, socket) do
    table = socket.assigns.table
    events = Map.get(payload, "events", ["*"])
    filter_string = Map.get(payload, "filter", nil)
    filters = Realtime.Filter.parse_filter(filter_string)
    ref = Map.get(payload, "ref", nil)

    Realtime.SubscriptionRegistry.subscribe(self(), table, events, filters, socket.assigns.user_id)

    push(socket, "subscribed", %{"ref" => ref, "table" => table})
    {:noreply, socket}
  end

  @impl true
  def handle_in("unsubscribe", payload, socket) do
    table = socket.assigns.table
    ref = Map.get(payload, "ref", nil)

    Realtime.SubscriptionRegistry.unsubscribe(self(), table)

    push(socket, "unsubscribed", %{"ref" => ref, "table" => table})
    {:noreply, socket}
  end

  @impl true
  def handle_in("heartbeat", payload, socket) do
    ref = Map.get(payload, "ref", nil)
    push(socket, "heartbeat_ack", %{"ref" => ref})
    {:noreply, socket}
  end

  @impl true
  def handle_info({:realtime_change, change}, socket) do
    push(socket, "change", %{
      "table" => change["table"],
      "event" => change["event"],
      "new" => change["new"],
      "old" => change["old"],
      "timestamp" => change["timestamp"],
      "truncated" => Map.get(change, "truncated", false)
    })

    {:noreply, socket}
  end

  @impl true
  def terminate(_reason, _socket) do
    Realtime.SubscriptionRegistry.unsubscribe_all(self())
    :ok
  end
end

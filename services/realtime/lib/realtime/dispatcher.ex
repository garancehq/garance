defmodule Realtime.Dispatcher do
  use GenServer
  require Logger

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_) do
    Phoenix.PubSub.subscribe(Realtime.PubSub, "pg_changes")
    {:ok, %{}}
  end

  @impl true
  def handle_info({:pg_change, change}, state) do
    table = change["table"]
    subscribers = Realtime.SubscriptionRegistry.get_subscribers(table)

    for sub <- subscribers do
      if Realtime.Filter.match?(change, sub) do
        send(sub.pid, {:realtime_change, change})
      end
    end

    {:noreply, state}
  end
end

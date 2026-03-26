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
      if Realtime.Filter.match?(change, sub) and has_permission?(change, sub) do
        send(sub.pid, {:realtime_change, change})
      end
    end

    {:noreply, state}
  end

  defp has_permission?(_change, %{user_id: nil}), do: true

  defp has_permission?(change, %{user_id: user_id, filters: filters}) do
    row = change["new"] || change["old"] || %{}

    case find_owner_column(filters) do
      nil -> true
      column -> to_string(row[column]) == to_string(user_id)
    end
  end

  defp find_owner_column(filters) do
    Enum.find_value(filters, fn
      {column, "eq", _value} -> column
      _ -> nil
    end)
  end
end

defmodule Realtime.SubscriptionRegistry do
  use GenServer

  @table :subscriptions

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_) do
    :ets.new(@table, [:bag, :named_table, :public, read_concurrency: true])
    {:ok, %{}}
  end

  @doc """
  Register a subscription.
  - pid: the channel process
  - table: table name (e.g., "todos")
  - events: list of event types (["INSERT", "UPDATE", "DELETE", "*"])
  - filters: list of {column, operator, value} tuples
  """
  def subscribe(pid, table, events, filters \\ []) do
    ref = Process.monitor(pid)
    :ets.insert(@table, {pid, ref, table, events, filters})
    :ok
  end

  @doc "Remove all subscriptions for a pid on a specific table."
  def unsubscribe(pid, table) do
    entries = :ets.match_object(@table, {pid, :_, table, :_, :_})

    for {_pid, ref, _table, _events, _filters} <- entries do
      Process.demonitor(ref, [:flush])
    end

    :ets.match_delete(@table, {pid, :_, table, :_, :_})
    :ok
  end

  @doc "Remove all subscriptions for a pid (on disconnect)."
  def unsubscribe_all(pid) do
    entries = :ets.match_object(@table, {pid, :_, :_, :_, :_})

    for {_pid, ref, _table, _events, _filters} <- entries do
      Process.demonitor(ref, [:flush])
    end

    :ets.match_delete(@table, {pid, :_, :_, :_, :_})
    :ok
  end

  @doc "Get all subscriptions that match a given table."
  def get_subscribers(table) do
    :ets.match_object(@table, {:_, :_, table, :_, :_})
    |> Enum.map(fn {pid, _ref, _table, events, filters} ->
      %{pid: pid, events: events, filters: filters}
    end)
  end

  @impl true
  def handle_info({:DOWN, _ref, :process, pid, _reason}, state) do
    unsubscribe_all(pid)
    {:noreply, state}
  end
end

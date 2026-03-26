defmodule Realtime.SubscriptionRegistry do
  use GenServer

  @table :subscriptions

  def start_link(_opts) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @impl true
  def init(_) do
    case :ets.info(@table) do
      :undefined ->
        :ets.new(@table, [:bag, :named_table, :public, read_concurrency: true])

      _info ->
        # La table existe déjà (e.g. redémarrage du GenServer) : on la vide.
        :ets.delete_all_objects(@table)
    end

    {:ok, %{}}
  end

  @doc """
  Register a subscription.
  - pid: the channel process
  - table: table name (e.g., "todos")
  - events: list of event types (["INSERT", "UPDATE", "DELETE", "*"])
  - filters: list of {column, operator, value} tuples
  - user_id: optional user identifier for permission filtering
  """
  def subscribe(pid, table, events, filters \\ [], user_id \\ nil) do
    ref = Process.monitor(pid)
    :ets.insert(@table, {pid, ref, table, events, filters, user_id})
    :ok
  end

  @doc "Remove all subscriptions for a pid on a specific table."
  def unsubscribe(pid, table) do
    entries = :ets.match_object(@table, {pid, :_, table, :_, :_, :_})

    for {_pid, ref, _table, _events, _filters, _user_id} <- entries do
      Process.demonitor(ref, [:flush])
    end

    :ets.match_delete(@table, {pid, :_, table, :_, :_, :_})
    :ok
  end

  @doc "Remove all subscriptions for a pid (on disconnect)."
  def unsubscribe_all(pid) do
    entries = :ets.match_object(@table, {pid, :_, :_, :_, :_, :_})

    for {_pid, ref, _table, _events, _filters, _user_id} <- entries do
      Process.demonitor(ref, [:flush])
    end

    :ets.match_delete(@table, {pid, :_, :_, :_, :_, :_})
    :ok
  end

  @doc "Get all subscriptions that match a given table."
  def get_subscribers(table) do
    :ets.match_object(@table, {:_, :_, table, :_, :_, :_})
    |> Enum.map(fn {pid, _ref, _table, events, filters, user_id} ->
      %{pid: pid, events: events, filters: filters, user_id: user_id}
    end)
  end

  @impl true
  def handle_info({:DOWN, _ref, :process, pid, _reason}, state) do
    unsubscribe_all(pid)
    {:noreply, state}
  end
end

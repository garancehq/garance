defmodule Realtime.PgListener do
  use GenServer
  require Logger

  @channel "garance_changes"
  @reconnect_interval 5_000

  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  @doc """
  Parse a PostgreSQL URL into Postgrex connection options.
  """
  def parse_database_url(url) do
    uri = URI.parse(url)

    opts = [
      hostname: uri.host || "localhost",
      port: uri.port || 5432,
      database: (uri.path || "/") |> String.trim_leading("/")
    ]

    opts =
      case uri.userinfo do
        nil ->
          opts

        userinfo ->
          case String.split(userinfo, ":", parts: 2) do
            [username, password] -> opts ++ [username: username, password: password]
            [username] -> opts ++ [username: username]
          end
      end

    opts
  end

  @impl true
  def init(opts) do
    database_url = Keyword.fetch!(opts, :database_url)
    send(self(), :connect)
    {:ok, %{database_url: database_url, conn: nil}}
  end

  @impl true
  def handle_info(:connect, state) do
    pg_opts = parse_database_url(state.database_url) ++ [name: __MODULE__.Notifications]

    case Postgrex.Notifications.start_link(pg_opts) do
      {:ok, pid} ->
        Postgrex.Notifications.listen!(pid, @channel)
        Logger.info("PgListener connected, listening on #{@channel}")
        {:noreply, %{state | conn: pid}}

      {:error, reason} ->
        Logger.error("PgListener connection failed: #{inspect(reason)}, retrying in #{@reconnect_interval}ms")
        Process.send_after(self(), :connect, @reconnect_interval)
        {:noreply, state}
    end
  end

  @impl true
  def handle_info({:notification, _pid, _ref, @channel, payload}, state) do
    case Jason.decode(payload) do
      {:ok, change} ->
        Phoenix.PubSub.broadcast(
          Realtime.PubSub,
          "pg_changes",
          {:pg_change, change}
        )

      {:error, reason} ->
        Logger.warning("Failed to decode notification: #{inspect(reason)}")
    end

    {:noreply, state}
  end

  @impl true
  def handle_info({:DOWN, _ref, :process, _pid, reason}, state) do
    Logger.warning("PgListener connection lost: #{inspect(reason)}, reconnecting...")
    Process.send_after(self(), :connect, @reconnect_interval)
    {:noreply, %{state | conn: nil}}
  end

  @impl true
  def handle_info(msg, state) do
    Logger.debug("PgListener received unexpected message: #{inspect(msg)}")
    {:noreply, state}
  end
end

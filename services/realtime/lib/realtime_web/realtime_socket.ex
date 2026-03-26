defmodule RealtimeWeb.RealtimeSocket do
  use Phoenix.Socket

  # Channels will be added in Task 3
  # channel "realtime:*", RealtimeWeb.ChangesChannel

  @impl true
  def connect(_params, socket, _connect_info) do
    {:ok, socket}
  end

  @impl true
  def id(_socket), do: nil
end

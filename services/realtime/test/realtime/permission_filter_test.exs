defmodule Realtime.PermissionFilterTest do
  use ExUnit.Case

  # -----------------------------------------------------------------------
  # Ces tests vérifient la logique de filtrage par permission en reproduisant
  # localement la logique de Realtime.Dispatcher.has_permission?/2.
  # Ils sont intentionnellement autonomes (sans dépendance à l'ETS ou au
  # GenServer SubscriptionRegistry) pour pouvoir s'exécuter sans l'OTP app.
  # -----------------------------------------------------------------------

  test "subscriber avec user_id ne reçoit que les changements qui lui appartiennent" do
    filters = Realtime.Filter.parse_filter("user_id=eq.user-1")
    sub = %{pid: self(), events: ["*"], filters: filters, user_id: "user-1"}

    change_user1 = %{
      "table" => "todos",
      "schema" => "public",
      "event" => "INSERT",
      "new" => %{"id" => "1", "user_id" => "user-1", "title" => "Todo de user-1"},
      "old" => nil,
      "timestamp" => "2026-03-26T12:00:00Z"
    }

    change_user2 = %{
      "table" => "todos",
      "schema" => "public",
      "event" => "INSERT",
      "new" => %{"id" => "2", "user_id" => "user-2", "title" => "Todo de user-2"},
      "old" => nil,
      "timestamp" => "2026-03-26T12:00:01Z"
    }

    # user-1 doit passer le filtre de permission
    assert Realtime.Filter.match?(change_user1, sub)
    assert permission_ok?(change_user1, sub)

    # user-2 doit être bloqué par le filtre de permission (row ne correspond pas)
    assert Realtime.Filter.match?(change_user2, sub) == false
    assert permission_ok?(change_user2, sub) == false
  end

  test "subscriber sans user_id reçoit tous les changements" do
    sub = %{pid: self(), events: ["*"], filters: [], user_id: nil}

    change_user1 = %{
      "table" => "todos",
      "event" => "INSERT",
      "new" => %{"id" => "1", "user_id" => "user-1"},
      "old" => nil,
      "timestamp" => "2026-03-26T12:00:00Z"
    }

    change_user2 = %{
      "table" => "todos",
      "event" => "INSERT",
      "new" => %{"id" => "2", "user_id" => "user-2"},
      "old" => nil,
      "timestamp" => "2026-03-26T12:00:01Z"
    }

    assert permission_ok?(change_user1, sub)
    assert permission_ok?(change_user2, sub)
  end

  test "subscriber avec user_id mais sans filtre reçoit tous les changements" do
    # user_id présent mais pas de filtre eq sur une colonne owner → pas de restriction
    sub = %{pid: self(), events: ["INSERT"], filters: [], user_id: "user-1"}

    change = %{
      "table" => "todos",
      "event" => "INSERT",
      "new" => %{"id" => "3", "user_id" => "user-2"},
      "old" => nil,
      "timestamp" => "2026-03-26T12:00:00Z"
    }

    assert permission_ok?(change, sub)
  end

  test "subscriber avec user_id vérifie le bon champ de la ligne pour DELETE" do
    filters = Realtime.Filter.parse_filter("owner_id=eq.user-1")
    sub = %{pid: self(), events: ["DELETE"], filters: filters, user_id: "user-1"}

    change_own = %{
      "table" => "todos",
      "event" => "DELETE",
      "new" => nil,
      "old" => %{"id" => "10", "owner_id" => "user-1"},
      "timestamp" => "2026-03-26T12:00:00Z"
    }

    change_other = %{
      "table" => "todos",
      "event" => "DELETE",
      "new" => nil,
      "old" => %{"id" => "11", "owner_id" => "user-99"},
      "timestamp" => "2026-03-26T12:00:01Z"
    }

    assert permission_ok?(change_own, sub)
    refute permission_ok?(change_other, sub)
  end

  # -----------------------------------------------------------------------
  # Réplication locale de la logique has_permission?/2 du Dispatcher
  # -----------------------------------------------------------------------

  defp permission_ok?(_change, %{user_id: nil}), do: true

  defp permission_ok?(change, %{user_id: user_id, filters: filters}) do
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

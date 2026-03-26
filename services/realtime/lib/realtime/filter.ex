defmodule Realtime.Filter do
  @moduledoc "Match a PG change payload against subscription criteria."

  @doc "Returns true if the change matches the subscription."
  def match?(change, %{events: events, filters: filters}) do
    event_matches?(change["event"], events) and
      filters_match?(change, filters)
  end

  defp event_matches?(_event, ["*"]), do: true
  defp event_matches?(event, events), do: event in events

  defp filters_match?(_change, []), do: true

  defp filters_match?(change, filters) do
    # For INSERT/UPDATE, filter against "new". For DELETE, filter against "old".
    row = change["new"] || change["old"] || %{}

    Enum.all?(filters, fn {column, op, value} ->
      compare(row[column], op, value)
    end)
  end

  defp compare(nil, _op, _value), do: false
  defp compare(actual, "eq", expected), do: to_string(actual) == to_string(expected)
  defp compare(actual, "neq", expected), do: to_string(actual) != to_string(expected)
  defp compare(actual, "gt", expected), do: to_string(actual) > to_string(expected)
  defp compare(actual, "gte", expected), do: to_string(actual) >= to_string(expected)
  defp compare(actual, "lt", expected), do: to_string(actual) < to_string(expected)
  defp compare(actual, "lte", expected), do: to_string(actual) <= to_string(expected)
  defp compare(_actual, _op, _expected), do: false

  @doc "Parse a filter string like 'user_id=eq.123,status=eq.active' into tuples."
  def parse_filter(nil), do: []
  def parse_filter(""), do: []

  def parse_filter(filter_string) do
    filter_string
    |> String.split(",")
    |> Enum.map(&parse_single_filter/1)
    |> Enum.filter(&(&1 != nil))
  end

  defp parse_single_filter(filter) do
    case String.split(filter, "=", parts: 2) do
      [column, op_value] ->
        case String.split(op_value, ".", parts: 2) do
          [op, value] -> {column, op, value}
          _ -> nil
        end

      _ ->
        nil
    end
  end
end

defmodule Realtime.FilterTest do
  use ExUnit.Case

  test "wildcard event matches all events" do
    change = %{"event" => "INSERT", "new" => %{}, "old" => nil}
    sub = %{events: ["*"], filters: []}
    assert Realtime.Filter.match?(change, sub)
  end

  test "specific event type filters correctly" do
    insert = %{"event" => "INSERT", "new" => %{}, "old" => nil}
    delete = %{"event" => "DELETE", "new" => nil, "old" => %{}}
    sub = %{events: ["INSERT"], filters: []}

    assert Realtime.Filter.match?(insert, sub)
    refute Realtime.Filter.match?(delete, sub)
  end

  test "column filter eq matches" do
    change = %{"event" => "INSERT", "new" => %{"user_id" => "123", "title" => "test"}, "old" => nil}
    sub_match = %{events: ["*"], filters: [{"user_id", "eq", "123"}]}
    sub_no_match = %{events: ["*"], filters: [{"user_id", "eq", "456"}]}

    assert Realtime.Filter.match?(change, sub_match)
    refute Realtime.Filter.match?(change, sub_no_match)
  end

  test "column filter on DELETE uses old row" do
    change = %{"event" => "DELETE", "new" => nil, "old" => %{"user_id" => "123"}}
    sub = %{events: ["*"], filters: [{"user_id", "eq", "123"}]}
    assert Realtime.Filter.match?(change, sub)
  end

  test "multiple filters require all to match" do
    change = %{"event" => "INSERT", "new" => %{"user_id" => "123", "status" => "active"}, "old" => nil}
    sub = %{events: ["*"], filters: [{"user_id", "eq", "123"}, {"status", "eq", "active"}]}
    assert Realtime.Filter.match?(change, sub)

    sub_partial = %{events: ["*"], filters: [{"user_id", "eq", "123"}, {"status", "eq", "done"}]}
    refute Realtime.Filter.match?(change, sub_partial)
  end

  test "parse_filter parses PostgREST-style filter string" do
    assert Realtime.Filter.parse_filter("user_id=eq.123") == [{"user_id", "eq", "123"}]

    assert Realtime.Filter.parse_filter("user_id=eq.123,status=eq.active") == [
             {"user_id", "eq", "123"},
             {"status", "eq", "active"}
           ]

    assert Realtime.Filter.parse_filter("") == []
    assert Realtime.Filter.parse_filter(nil) == []
  end
end

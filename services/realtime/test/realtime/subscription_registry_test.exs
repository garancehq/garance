defmodule Realtime.SubscriptionRegistryTest do
  use ExUnit.Case

  setup do
    # Registry is started by the application
    :ok
  end

  test "subscribe and get_subscribers" do
    pid = self()
    Realtime.SubscriptionRegistry.subscribe(pid, "todos", ["INSERT"], [])
    subs = Realtime.SubscriptionRegistry.get_subscribers("todos")
    assert length(subs) == 1
    assert hd(subs).pid == pid
    assert hd(subs).events == ["INSERT"]

    # Cleanup
    Realtime.SubscriptionRegistry.unsubscribe_all(pid)
  end

  test "unsubscribe removes subscription" do
    pid = self()
    Realtime.SubscriptionRegistry.subscribe(pid, "todos", ["*"], [])
    Realtime.SubscriptionRegistry.unsubscribe(pid, "todos")
    assert Realtime.SubscriptionRegistry.get_subscribers("todos") == []
  end

  test "unsubscribe_all removes all subscriptions for a pid" do
    pid = self()
    Realtime.SubscriptionRegistry.subscribe(pid, "todos", ["INSERT"], [])
    Realtime.SubscriptionRegistry.subscribe(pid, "users", ["UPDATE"], [])
    Realtime.SubscriptionRegistry.unsubscribe_all(pid)
    assert Realtime.SubscriptionRegistry.get_subscribers("todos") == []
    assert Realtime.SubscriptionRegistry.get_subscribers("users") == []
  end

  test "different tables are independent" do
    pid = self()
    Realtime.SubscriptionRegistry.subscribe(pid, "todos", ["INSERT"], [])
    assert Realtime.SubscriptionRegistry.get_subscribers("users") == []

    Realtime.SubscriptionRegistry.unsubscribe_all(pid)
  end
end

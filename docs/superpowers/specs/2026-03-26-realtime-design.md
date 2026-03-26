# Realtime — Design Spec

> Design document — March 26, 2026

## Goal

Add real-time subscriptions to Garance. Clients connect via WebSocket and receive live INSERT/UPDATE/DELETE events on database tables, with server-side filtering by event type and column values. Built in Elixir/Phoenix for massive concurrency.

## Decisions

- **Language:** Elixir/Phoenix (same choice as Supabase Realtime)
- **PG mechanism:** LISTEN/NOTIFY via triggers
- **Client protocol:** WebSocket (Phoenix Channels)
- **Subscription granularity:** table + event type + column filter
- **Payload:** complete (new + old row), truncated if > 8KB

## 1. Architecture

```
Client (SDK)
  │ ws://gateway:8080/realtime
  ▼
Gateway (Go) ──WS proxy──▶ Realtime (Elixir/Phoenix :5003)
                                │
                                ├─ PgListener (GenServer, LISTEN garance_changes)
                                ├─ SubscriptionRegistry (ETS)
                                ├─ Filter (match payload vs subscriptions)
                                └─ ChangesChannel (Phoenix Channel, push to clients)
                                        │
                                        ▼
                                   PostgreSQL
                              (trigger → NOTIFY garance_changes)
```

**Ports:**
- HTTP 4003 (health check)
- WebSocket 5003 (Phoenix Channels)

## 2. PostgreSQL Trigger

A single reusable trigger function, attached to user tables automatically.

```sql
CREATE OR REPLACE FUNCTION garance_notify_change() RETURNS trigger AS $$
DECLARE
  payload json;
  payload_text text;
BEGIN
  payload = json_build_object(
    'table', TG_TABLE_NAME,
    'schema', TG_TABLE_SCHEMA,
    'event', TG_OP,
    'new', CASE WHEN TG_OP IN ('INSERT', 'UPDATE') THEN row_to_json(NEW) ELSE NULL END,
    'old', CASE WHEN TG_OP IN ('UPDATE', 'DELETE') THEN row_to_json(OLD) ELSE NULL END,
    'timestamp', now()
  );

  payload_text = payload::text;

  -- Truncate if > 7KB (leave room for NOTIFY overhead)
  IF length(payload_text) > 7168 THEN
    payload = json_build_object(
      'table', TG_TABLE_NAME,
      'schema', TG_TABLE_SCHEMA,
      'event', TG_OP,
      'new', NULL,
      'old', NULL,
      'timestamp', now(),
      'truncated', true
    );
    payload_text = payload::text;
  END IF;

  PERFORM pg_notify('garance_changes', payload_text);

  RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
```

**Trigger attachment:** managed by the Engine. When `_reload` or `_migrate/apply` runs, the Engine attaches the trigger to all user tables that don't have it yet:

```sql
CREATE TRIGGER garance_realtime_trigger
  AFTER INSERT OR UPDATE OR DELETE ON "{table}"
  FOR EACH ROW EXECUTE FUNCTION garance_notify_change();
```

The Engine also creates the function `garance_notify_change()` if it doesn't exist (idempotent, `CREATE OR REPLACE`).

Tables in internal schemas (`garance_auth`, `garance_storage`, `garance_platform`, `garance_audit`) do NOT get the trigger.

## 3. Realtime Service (Elixir/Phoenix)

### Project Structure

```
services/realtime/
├── mix.exs
├── lib/
│   ├── realtime/
│   │   ├── application.ex          # Supervision tree
│   │   ├── pg_listener.ex          # LISTEN garance_changes, parse, dispatch
│   │   ├── subscription_registry.ex # ETS-backed subscription storage
│   │   └── filter.ex               # Match payload against subscription criteria
│   └── realtime_web/
│       ├── endpoint.ex
│       ├── channels/
│       │   └── changes_channel.ex  # Phoenix Channel — subscribe/unsubscribe/push
│       └── controllers/
│           └── health_controller.ex
├── config/
│   ├── config.exs
│   └── runtime.exs                 # DATABASE_URL, SECRET_KEY_BASE from env
├── Dockerfile
└── test/
    ├── realtime/
    │   ├── pg_listener_test.exs
    │   ├── subscription_registry_test.exs
    │   └── filter_test.exs
    └── realtime_web/
        └── channels/
            └── changes_channel_test.exs
```

### Components

**PgListener (GenServer):**
- Maintains a single persistent PG connection via `Postgrex.Notifications`
- Executes `LISTEN garance_changes` on startup
- On each notification: parse JSON payload → lookup matching subscriptions in Registry → push to each matching channel pid
- Auto-reconnects on PG connection loss (exponential backoff)

**SubscriptionRegistry (GenServer + ETS):**
- ETS table: `{channel_pid, table, event_filter, column_filters}`
- `event_filter`: list of event types (`["INSERT"]`, `["INSERT", "UPDATE"]`, or `["*"]`)
- `column_filters`: list of `{column, operator, value}` tuples (e.g., `{"user_id", "eq", "123"}`)
- Monitors channel pids — auto-removes subscriptions on disconnect

**Filter:**
- Pure functions, no state
- `match?(payload, subscription)` → boolean
- Checks: table name matches, event type matches (or `*`), all column filters match against `payload.new` (for INSERT/UPDATE) or `payload.old` (for DELETE)
- Filter operators supported: `eq`, `neq`, `gt`, `gte`, `lt`, `lte` (same as the REST API)

**ChangesChannel (Phoenix Channel):**
- Topic: `realtime:{table}` (e.g., `realtime:todos`)
- `join` → registers in SubscriptionRegistry
- Handles incoming messages:
  - `subscribe` → add subscription with filters
  - `unsubscribe` → remove subscription
  - `heartbeat` → reply with `heartbeat_ack`
- Receives pushes from PgListener when a matching change occurs

### Supervision Tree

```
Application
├── Realtime.PgListener
├── Realtime.SubscriptionRegistry
└── RealtimeWeb.Endpoint (Phoenix)
```

## 4. WebSocket Protocol

### Client → Server

```json
{ "type": "subscribe", "ref": "1", "table": "todos", "events": ["INSERT"], "filter": "user_id=eq.123" }
{ "type": "unsubscribe", "ref": "2", "table": "todos" }
{ "type": "heartbeat", "ref": "3" }
```

- `events`: array of `"INSERT"`, `"UPDATE"`, `"DELETE"`, or `"*"`. Default: `["*"]`.
- `filter`: PostgREST-style filter string. Optional. Multiple filters comma-separated: `"user_id=eq.123,status=eq.active"`.
- `ref`: client-provided reference for correlating acks.

### Server → Client

```json
{ "type": "change", "table": "todos", "event": "INSERT", "new": {...}, "old": null, "timestamp": "2026-03-26T12:00:00Z" }
{ "type": "change", "table": "todos", "event": "UPDATE", "new": {...}, "old": {...}, "timestamp": "..." }
{ "type": "change", "table": "todos", "event": "DELETE", "new": null, "old": {...}, "timestamp": "..." }
{ "type": "change", "table": "todos", "event": "INSERT", "new": null, "old": null, "timestamp": "...", "truncated": true }
{ "type": "subscribed", "ref": "1", "table": "todos" }
{ "type": "unsubscribed", "ref": "2", "table": "todos" }
{ "type": "heartbeat_ack", "ref": "3" }
{ "type": "error", "ref": "1", "message": "table not found" }
```

## 5. SDK Integration

New `realtime` module on the Garance SDK client:

```typescript
const garance = createClient({ url: 'http://localhost:8080' })

// Subscribe with event type + filter
const channel = garance.realtime
  .channel('todos')
  .on('INSERT', { filter: 'user_id=eq.123' }, (payload) => {
    console.log('New todo:', payload.new)
  })
  .on('UPDATE', (payload) => {
    console.log('Updated:', payload.old, '→', payload.new)
  })
  .on('DELETE', (payload) => {
    console.log('Deleted:', payload.old)
  })
  .on('*', (payload) => {
    // All events
  })
  .subscribe()

// Unsubscribe
channel.unsubscribe()

// Disconnect all channels
garance.realtime.disconnect()
```

**Payload type:**

```typescript
interface RealtimePayload<T = Record<string, unknown>> {
  event: 'INSERT' | 'UPDATE' | 'DELETE'
  table: string
  schema: string
  new: T | null
  old: T | null
  timestamp: string
  truncated?: boolean
}
```

**Reconnection:** exponential backoff (1s, 2s, 4s, 8s, max 30s). Subscriptions are re-sent after reconnection.

**Heartbeat:** client sends heartbeat every 30s. If no heartbeat_ack within 10s, considers connection dead and reconnects.

## 6. Gateway WebSocket Proxy

The Gateway proxies WebSocket connections from `/realtime` to the Realtime service.

```go
mux.HandleFunc("/realtime", p.ProxyRealtimeWebSocket)
```

Uses Go's `nhooyr.io/websocket` library for bidirectional WebSocket proxying. The Gateway does NOT inspect or modify WebSocket frames — pure pass-through.

Environment: `REALTIME_WS_URL=ws://realtime:5003/socket/websocket`

## 7. Docker Integration

### docker-compose.dev.yml addition

```yaml
realtime:
  build:
    context: ../services/realtime
    dockerfile: Dockerfile
  depends_on:
    postgres:
      condition: service_healthy
  environment:
    DATABASE_URL: postgresql://postgres:postgres@postgres:5432/garance
    PORT: "4003"
    WS_PORT: "5003"
    SECRET_KEY_BASE: dev-secret-key-base-at-least-64-chars-long-for-phoenix-to-accept-it
```

### Dockerfile

```dockerfile
FROM elixir:1.18-alpine AS builder
RUN apk add --no-cache build-base git
WORKDIR /app
ENV MIX_ENV=prod
COPY mix.exs mix.lock ./
RUN mix local.hex --force && mix local.rebar --force && mix deps.get --only prod && mix deps.compile
COPY lib lib
COPY config config
RUN mix compile && mix release

FROM alpine:3.20
RUN apk add --no-cache libstdc++ openssl ncurses-libs
COPY --from=builder /app/_build/prod/rel/realtime /app
ENV PORT=4003 WS_PORT=5003
EXPOSE 4003 5003
CMD ["/app/bin/realtime", "start"]
```

## 8. Engine Changes

The Engine must attach the realtime trigger to user tables. Changes to the Engine:

1. **Create trigger function** on startup (idempotent `CREATE OR REPLACE FUNCTION garance_notify_change()`)
2. **Attach trigger** after `_reload` and `_migrate/apply` — for each user table, run:
   ```sql
   DO $$
   BEGIN
     IF NOT EXISTS (
       SELECT 1 FROM pg_trigger WHERE tgname = 'garance_realtime_trigger'
       AND tgrelid = '"{table}"'::regclass
     ) THEN
       CREATE TRIGGER garance_realtime_trigger
         AFTER INSERT OR UPDATE OR DELETE ON "{table}"
         FOR EACH ROW EXECUTE FUNCTION garance_notify_change();
     END IF;
   END $$;
   ```
3. Skip tables in internal schemas (`garance_auth`, `garance_storage`, `garance_platform`, `garance_audit`)

## 9. Implementation Scope

| Component | Changes |
|---|---|
| New: `services/realtime/` | Full Elixir/Phoenix app (PgListener, SubscriptionRegistry, Filter, ChangesChannel) |
| Engine: `api/routes.rs` | Attach realtime triggers after reload/migrate |
| Engine: `main.rs` | Create trigger function on startup |
| Gateway: `proxy/realtime.go` | WebSocket proxy to Realtime service |
| Gateway: `main.go` | Add REALTIME_WS_URL config |
| SDK: `src/realtime.ts` | New RealtimeClient with channel/subscribe/reconnect |
| SDK: `src/index.ts` | Export realtime module from createClient |
| Deploy: `docker-compose.dev.yml` | Add realtime service |
| Deploy: `docker-compose.yml` | Add realtime service (published image) |

## 10. Testing

| Test | Component | Verifies |
|---|---|---|
| `test_pg_listener_receives_notify` | PgListener | NOTIFY → message received and parsed |
| `test_subscription_registry` | Registry | Add/remove/lookup subscriptions, auto-cleanup on pid down |
| `test_filter_event_type` | Filter | INSERT sub doesn't receive DELETE |
| `test_filter_column_value` | Filter | `user_id=eq.123` matches correctly |
| `test_filter_wildcard` | Filter | `*` receives all events |
| `test_channel_subscribe_unsubscribe` | Channel | Full flow: join → subscribe → receive → unsubscribe → stop |
| `test_payload_truncation` | PgListener | Large row → truncated payload with `truncated: true` |
| `test_reconnection` | PgListener | PG connection lost → auto-reconnect and re-LISTEN |

type EventType = 'INSERT' | 'UPDATE' | 'DELETE' | '*'

interface RealtimePayload {
  event: 'INSERT' | 'UPDATE' | 'DELETE'
  table: string
  schema: string
  new: Record<string, unknown> | null
  old: Record<string, unknown> | null
  timestamp: string
  truncated?: boolean
}

type ChangeCallback = (payload: RealtimePayload) => void

interface ChannelSubscription {
  event: EventType
  filter?: string
  callback: ChangeCallback
}

export class RealtimeChannel {
  private subscriptions: ChannelSubscription[] = []
  private active = false

  constructor(
    private client: RealtimeClient,
    private table: string,
  ) {}

  on(event: EventType, callbackOrFilter: ChangeCallback | { filter: string }, callback?: ChangeCallback): this {
    if (typeof callbackOrFilter === 'function') {
      this.subscriptions.push({ event, callback: callbackOrFilter })
    } else {
      this.subscriptions.push({ event, filter: callbackOrFilter.filter, callback: callback! })
    }
    return this
  }

  subscribe(): this {
    this.active = true
    this.client._subscribe(this)
    return this
  }

  unsubscribe(): void {
    this.active = false
    this.client._unsubscribe(this)
  }

  get tableName() { return this.table }
  get isActive() { return this.active }
  get handlers() { return this.subscriptions }
}

export class RealtimeClient {
  private ws: WebSocket | null = null
  private channels: Map<string, RealtimeChannel> = new Map()
  private reconnectAttempts = 0
  private maxReconnectDelay = 30000
  private heartbeatInterval: ReturnType<typeof setInterval> | null = null
  private refCounter = 0

  constructor(private baseUrl: string) {}

  channel(table: string): RealtimeChannel {
    const ch = new RealtimeChannel(this, table)
    return ch
  }

  _subscribe(channel: RealtimeChannel): void {
    this.channels.set(channel.tableName, channel)
    this.ensureConnected()

    for (const sub of channel.handlers) {
      this.send({
        type: 'subscribe',
        ref: String(++this.refCounter),
        table: channel.tableName,
        events: [sub.event],
        filter: sub.filter || '',
      })
    }
  }

  _unsubscribe(channel: RealtimeChannel): void {
    this.send({
      type: 'unsubscribe',
      ref: String(++this.refCounter),
      table: channel.tableName,
    })
    this.channels.delete(channel.tableName)
    if (this.channels.size === 0) {
      this.disconnect()
    }
  }

  disconnect(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval)
      this.heartbeatInterval = null
    }
    if (this.ws) {
      this.ws.close(1000)
      this.ws = null
    }
    this.channels.clear()
  }

  private ensureConnected(): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) return

    const wsUrl = this.baseUrl.replace(/^http/, 'ws') + '/realtime'
    this.ws = new WebSocket(wsUrl)

    this.ws.onopen = () => {
      this.reconnectAttempts = 0
      this.startHeartbeat()
      // Re-subscribe all channels
      for (const channel of this.channels.values()) {
        for (const sub of channel.handlers) {
          this.send({
            type: 'subscribe',
            ref: String(++this.refCounter),
            table: channel.tableName,
            events: [sub.event],
            filter: sub.filter || '',
          })
        }
      }
    }

    this.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)
        this.handleMessage(msg)
      } catch {
        // Ignore malformed messages
      }
    }

    this.ws.onclose = () => {
      this.ws = null
      if (this.channels.size > 0) {
        this.reconnect()
      }
    }

    this.ws.onerror = () => {
      // onclose will fire after onerror
    }
  }

  private handleMessage(msg: Record<string, unknown>): void {
    if (msg.type === 'change') {
      const table = msg.table as string
      const channel = this.channels.get(table)
      if (!channel) return

      const payload: RealtimePayload = {
        event: msg.event as RealtimePayload['event'],
        table: msg.table as string,
        schema: (msg.schema as string) || 'public',
        new: msg.new as Record<string, unknown> | null,
        old: msg.old as Record<string, unknown> | null,
        timestamp: msg.timestamp as string,
        truncated: msg.truncated as boolean | undefined,
      }

      for (const sub of channel.handlers) {
        if (sub.event === '*' || sub.event === payload.event) {
          sub.callback(payload)
        }
      }
    }
  }

  private reconnect(): void {
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), this.maxReconnectDelay)
    this.reconnectAttempts++
    setTimeout(() => this.ensureConnected(), delay)
  }

  private startHeartbeat(): void {
    if (this.heartbeatInterval) clearInterval(this.heartbeatInterval)
    this.heartbeatInterval = setInterval(() => {
      this.send({ type: 'heartbeat', ref: String(++this.refCounter) })
    }, 30000)
  }

  private send(msg: Record<string, unknown>): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg))
    }
  }
}

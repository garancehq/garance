import type { GaranceClientOptions, GaranceError, GaranceResult } from './types'

export class HttpClient {
  private baseUrl: string
  private headers: Record<string, string>
  private accessToken: string | null = null

  constructor(options: GaranceClientOptions) {
    this.baseUrl = options.url.replace(/\/$/, '')
    this.headers = options.headers ?? {}
  }

  setAccessToken(token: string | null) {
    this.accessToken = token
  }

  getAccessToken(): string | null {
    return this.accessToken
  }

  getBaseUrl(): string {
    return this.baseUrl
  }

  private buildHeaders(extra?: Record<string, string>): Record<string, string> {
    const headers: Record<string, string> = { ...this.headers, ...extra }
    if (this.accessToken) {
      headers['Authorization'] = `Bearer ${this.accessToken}`
    }
    return headers
  }

  async get<T>(path: string, params?: Record<string, string>): Promise<GaranceResult<T>> {
    const url = new URL(`${this.baseUrl}${path}`)
    if (params) {
      for (const [k, v] of Object.entries(params)) {
        url.searchParams.set(k, v)
      }
    }
    return this.request<T>(url.toString(), { method: 'GET' })
  }

  async post<T>(path: string, body?: unknown): Promise<GaranceResult<T>> {
    return this.request<T>(`${this.baseUrl}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async patch<T>(path: string, body: unknown): Promise<GaranceResult<T>> {
    return this.request<T>(`${this.baseUrl}${path}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  }

  async delete(path: string): Promise<GaranceResult<void>> {
    return this.request<void>(`${this.baseUrl}${path}`, { method: 'DELETE' })
  }

  async uploadFile(path: string, file: Blob | File, fileName?: string): Promise<GaranceResult<unknown>> {
    const formData = new FormData()
    formData.append('file', file, fileName)
    return this.request(`${this.baseUrl}${path}`, { method: 'POST', body: formData })
  }

  private async request<T>(url: string, init: RequestInit): Promise<GaranceResult<T>> {
    const headers = this.buildHeaders(init.headers as Record<string, string>)
    try {
      const response = await fetch(url, { ...init, headers })
      if (response.status === 204) {
        return { data: undefined as T, error: null }
      }
      const body = await response.json()
      if (!response.ok) {
        return { data: null, error: body as GaranceError }
      }
      return { data: body as T, error: null }
    } catch (err) {
      return { data: null, error: { error: { code: 'NETWORK_ERROR', message: String(err), status: 0 } } }
    }
  }
}

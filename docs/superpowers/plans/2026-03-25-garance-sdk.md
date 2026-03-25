# @garance/sdk — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `@garance/sdk` — the TypeScript client SDK for Garance. Provides `createClient()` with `.auth`, `.from()` (data), and `.storage` namespaces. Consumes the Gateway HTTP API.

**Architecture:** Lightweight HTTP client with typed namespaces. No external dependencies — uses native `fetch`. API design mirrors Supabase SDK for familiarity.

**Tech Stack:** TypeScript 5.x, tsup (bundling), vitest (testing)

**Spec:** `docs/superpowers/specs/2026-03-25-garance-baas-design.md` (sections 6, 7 SDK examples)

---

## Task 1: Package Setup & Core Client

**Files:**
- Create: `sdks/typescript/package.json`
- Create: `sdks/typescript/tsconfig.json`
- Create: `sdks/typescript/tsup.config.ts`
- Create: `sdks/typescript/src/index.ts`
- Create: `sdks/typescript/src/client.ts`
- Create: `sdks/typescript/src/types.ts`

- [ ] **Step 1: Create package**

```json
{
  "name": "@garance/sdk",
  "version": "0.1.0",
  "description": "Garance BaaS TypeScript SDK",
  "type": "module",
  "main": "./dist/index.cjs",
  "module": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": { "types": "./dist/index.d.ts", "import": "./dist/index.js", "require": "./dist/index.cjs" }
  },
  "scripts": { "build": "tsup", "test": "vitest run", "test:watch": "vitest" },
  "files": ["dist"],
  "license": "Apache-2.0",
  "devDependencies": { "tsup": "^8.0.0", "typescript": "^5.7.0", "vitest": "^3.0.0" }
}
```

- [ ] **Step 2: Write core types**

```typescript
// sdks/typescript/src/types.ts
export interface GaranceClientOptions {
  url: string
  headers?: Record<string, string>
}

export interface AuthTokens {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: string
}

export interface User {
  id: string
  email: string
  email_verified: boolean
  role: string
  created_at: string
  updated_at: string
}

export interface AuthResponse {
  user: User
  token_pair: AuthTokens
}

export interface GaranceError {
  error: { code: string; message: string; status: number }
}

export type GaranceResult<T> = { data: T; error: null } | { data: null; error: GaranceError }
```

- [ ] **Step 3: Write HTTP client**

```typescript
// sdks/typescript/src/client.ts
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
```

- [ ] **Step 4: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add sdks/
git commit -m ":tada: feat(sdk): initialize @garance/sdk with HTTP client"
```

---

## Task 2: Auth, Data & Storage Namespaces

**Files:**
- Create: `sdks/typescript/src/auth.ts`
- Create: `sdks/typescript/src/data.ts`
- Create: `sdks/typescript/src/storage.ts`
- Modify: `sdks/typescript/src/index.ts`
- Create: `sdks/typescript/src/__tests__/sdk.test.ts`

- [ ] **Step 1: Write auth namespace**

```typescript
// sdks/typescript/src/auth.ts
import type { HttpClient } from './client'
import type { AuthResponse, User } from './types'

export class AuthClient {
  constructor(private http: HttpClient) {}

  async signUp(params: { email: string; password: string }) {
    const result = await this.http.post<AuthResponse>('/auth/v1/signup', params)
    if (result.data?.token_pair) {
      this.http.setAccessToken(result.data.token_pair.access_token)
    }
    return result
  }

  async signIn(params: { email: string; password: string }) {
    const result = await this.http.post<AuthResponse>('/auth/v1/signin', params)
    if (result.data?.token_pair) {
      this.http.setAccessToken(result.data.token_pair.access_token)
    }
    return result
  }

  async signInWithMagicLink(params: { email: string }) {
    return this.http.post('/auth/v1/magic-link', params)
  }

  async signInWithOAuth(params: { provider: string }) {
    return this.http.post('/auth/v1/oauth', params)
  }

  async refreshToken(refreshToken: string) {
    const result = await this.http.post<AuthResponse>('/auth/v1/token/refresh', { refresh_token: refreshToken })
    if (result.data?.token_pair) {
      this.http.setAccessToken(result.data.token_pair.access_token)
    }
    return result
  }

  async signOut(refreshToken: string) {
    this.http.setAccessToken(null)
    return this.http.post('/auth/v1/signout', { refresh_token: refreshToken })
  }

  async getUser() {
    return this.http.get<User>('/auth/v1/user')
  }

  async deleteUser() {
    return this.http.delete('/auth/v1/user')
  }
}
```

- [ ] **Step 2: Write data namespace (query builder)**

```typescript
// sdks/typescript/src/data.ts
import type { HttpClient } from './client'

export class QueryBuilder<T = unknown> {
  private _table: string
  private _http: HttpClient
  private _params: Record<string, string> = {}
  private _selectCols?: string

  constructor(http: HttpClient, table: string) {
    this._http = http
    this._table = table
  }

  select(columns: string) {
    this._selectCols = columns
    return this
  }

  eq(column: string, value: string | number | boolean) {
    this._params[column] = `eq.${value}`
    return this
  }

  neq(column: string, value: string | number | boolean) {
    this._params[column] = `neq.${value}`
    return this
  }

  gt(column: string, value: string | number) {
    this._params[column] = `gt.${value}`
    return this
  }

  gte(column: string, value: string | number) {
    this._params[column] = `gte.${value}`
    return this
  }

  lt(column: string, value: string | number) {
    this._params[column] = `lt.${value}`
    return this
  }

  lte(column: string, value: string | number) {
    this._params[column] = `lte.${value}`
    return this
  }

  order(column: string, direction: 'asc' | 'desc' = 'asc') {
    this._params['order'] = `${column}.${direction}`
    return this
  }

  limit(n: number) {
    this._params['limit'] = String(n)
    return this
  }

  offset(n: number) {
    this._params['offset'] = String(n)
    return this
  }

  /** Execute the query — returns array of rows */
  async execute() {
    const params = { ...this._params }
    if (this._selectCols) params['select'] = this._selectCols
    return this._http.get<T[]>(`/api/v1/${this._table}`, params)
  }

  /** Get a single row by ID */
  async getById(id: string) {
    return this._http.get<T>(`/api/v1/${this._table}/${id}`)
  }

  /** Insert a new row */
  async insert(data: Partial<T>) {
    return this._http.post<T>(`/api/v1/${this._table}`, data)
  }

  /** Update a row by ID */
  async update(id: string, data: Partial<T>) {
    return this._http.patch<T>(`/api/v1/${this._table}/${id}`, data)
  }

  /** Delete a row by ID */
  async deleteById(id: string) {
    return this._http.delete(`/api/v1/${this._table}/${id}`)
  }
}
```

- [ ] **Step 3: Write storage namespace**

```typescript
// sdks/typescript/src/storage.ts
import type { HttpClient } from './client'

export class StorageClient {
  constructor(private http: HttpClient, private baseUrl: string) {}

  from(bucket: string) {
    return new BucketClient(this.http, this.baseUrl, bucket)
  }
}

class BucketClient {
  constructor(
    private http: HttpClient,
    private baseUrl: string,
    private bucket: string,
  ) {}

  async upload(path: string, file: Blob | File) {
    return this.http.uploadFile(`/storage/v1/${this.bucket}/upload`, file, path)
  }

  getPublicUrl(path: string, options?: { transform?: { width?: number; height?: number; format?: string } }) {
    let url = `${this.baseUrl}/storage/v1/${this.bucket}/${path}`
    if (options?.transform) {
      const params = new URLSearchParams()
      if (options.transform.width) params.set('w', String(options.transform.width))
      if (options.transform.height) params.set('h', String(options.transform.height))
      if (options.transform.format) params.set('f', options.transform.format)
      url += `?${params.toString()}`
    }
    return url
  }

  async createSignedUrl(path: string, options: { expiresIn: number }) {
    return this.http.post<{ signed_url: string }>(`/storage/v1/${this.bucket}/signed-url`, {
      file_name: path,
      expires_in: options.expiresIn,
    })
  }

  async list(options?: { prefix?: string; limit?: number; offset?: number }) {
    const params: Record<string, string> = {}
    if (options?.prefix) params['prefix'] = options.prefix
    if (options?.limit) params['limit'] = String(options.limit)
    if (options?.offset) params['offset'] = String(options.offset)
    return this.http.get(`/storage/v1/${this.bucket}`, params)
  }

  async remove(path: string) {
    return this.http.delete(`/storage/v1/${this.bucket}/${path}`)
  }
}
```

- [ ] **Step 4: Write main entry point**

```typescript
// sdks/typescript/src/index.ts
import { HttpClient } from './client'
import { AuthClient } from './auth'
import { QueryBuilder } from './data'
import { StorageClient } from './storage'
import type { GaranceClientOptions } from './types'

export function createClient(options: GaranceClientOptions) {
  const http = new HttpClient(options)

  return {
    auth: new AuthClient(http),
    from: <T = unknown>(table: string) => new QueryBuilder<T>(http, table),
    storage: new StorageClient(http, options.url.replace(/\/$/, '')),
  }
}

export type { GaranceClientOptions, AuthTokens, User, AuthResponse, GaranceError, GaranceResult } from './types'
export { HttpClient } from './client'
export { AuthClient } from './auth'
export { QueryBuilder } from './data'
export { StorageClient } from './storage'
```

- [ ] **Step 5: Write tests**

```typescript
// sdks/typescript/src/__tests__/sdk.test.ts
import { describe, it, expect } from 'vitest'
import { createClient } from '../index'

describe('createClient', () => {
  it('creates a client with auth, from, and storage namespaces', () => {
    const garance = createClient({ url: 'http://localhost:8080' })
    expect(garance.auth).toBeDefined()
    expect(garance.from).toBeInstanceOf(Function)
    expect(garance.storage).toBeDefined()
  })
})

describe('QueryBuilder', () => {
  it('builds a query with filters', () => {
    const garance = createClient({ url: 'http://localhost:8080' })
    const query = garance.from('users').eq('name', 'Alice').limit(10)
    expect(query).toBeDefined()
  })
})

describe('StorageClient', () => {
  it('generates public URL', () => {
    const garance = createClient({ url: 'http://localhost:8080' })
    const url = garance.storage.from('avatars').getPublicUrl('user-123/photo.jpg')
    expect(url).toBe('http://localhost:8080/storage/v1/avatars/user-123/photo.jpg')
  })

  it('generates public URL with transforms', () => {
    const garance = createClient({ url: 'http://localhost:8080' })
    const url = garance.storage.from('avatars').getPublicUrl('photo.jpg', {
      transform: { width: 200, height: 200, format: 'webp' },
    })
    expect(url).toContain('w=200')
    expect(url).toContain('h=200')
    expect(url).toContain('f=webp')
  })
})

describe('SDK matches spec examples', () => {
  it('has the same API shape as documented', () => {
    const garance = createClient({ url: 'https://mon-projet.garance.io' })

    // These should all be valid calls (we can't test network, but verify the API exists)
    expect(typeof garance.auth.signUp).toBe('function')
    expect(typeof garance.auth.signIn).toBe('function')
    expect(typeof garance.auth.signInWithMagicLink).toBe('function')
    expect(typeof garance.auth.signInWithOAuth).toBe('function')
    expect(typeof garance.auth.getUser).toBe('function')
    expect(typeof garance.auth.signOut).toBe('function')

    const query = garance.from('users')
    expect(typeof query.execute).toBe('function')
    expect(typeof query.insert).toBe('function')
    expect(typeof query.getById).toBe('function')

    const storage = garance.storage.from('avatars')
    expect(typeof storage.upload).toBe('function')
    expect(typeof storage.getPublicUrl).toBe('function')
    expect(typeof storage.createSignedUrl).toBe('function')
  })
})
```

- [ ] **Step 6: Add tsup config**

```typescript
// sdks/typescript/tsup.config.ts
import { defineConfig } from 'tsup'

export default defineConfig({
  entry: ['src/index.ts'],
  format: ['esm', 'cjs'],
  dts: true,
  clean: true,
  sourcemap: true,
})
```

- [ ] **Step 7: Run tests and build**

Run: `cd packages && npm install && npm test && npm run build`
Expected: 5 tests pass, build succeeds.

- [ ] **Step 8: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add sdks/
git commit -m ":sparkles: feat(sdk): add @garance/sdk with auth, data query builder, and storage client"
```

---

## Summary

| Task | Description | Tests |
|---|---|---|
| 1 | Package setup, HTTP client, types | 0 |
| 2 | Auth, data (QueryBuilder), storage namespaces + tests | 5 |
| **Total** | | **5** |

### Not in this plan (deferred)

- Realtime subscriptions
- Automatic token refresh
- Retry logic / offline support
- Type inference from garance.schema.ts (requires codegen integration)
- Dart, Swift, Kotlin SDKs
- npm publishing

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

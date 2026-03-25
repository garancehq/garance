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

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

  async signInWithOAuth(params: { provider: string; redirectUri?: string }) {
    const redirectUri =
      params.redirectUri || (typeof window !== 'undefined' ? window.location.href : '')
    const url = `${this.http.getBaseUrl()}/auth/v1/oauth/${params.provider}?redirect_uri=${encodeURIComponent(redirectUri)}`
    if (typeof window !== 'undefined') {
      window.location.href = url
    }
    return { data: null, error: null }
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

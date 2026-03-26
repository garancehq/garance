# OAuth Providers — Design Spec

> Design document — March 26, 2026

## Goal

Add OAuth2 authentication for Google, GitHub, and GitLab. Developers configure providers via the dashboard (credentials stored encrypted in DB). End-users sign in via the standard Authorization Code flow.

## Providers

| Provider | Authorize URL | Token URL | Profile URL | Scopes |
|---|---|---|---|---|
| Google | `accounts.google.com/o/oauth2/v2/auth` | `oauth2.googleapis.com/token` | `googleapis.com/oauth2/v2/userinfo` | `email profile` |
| GitHub | `github.com/login/oauth/authorize` | `github.com/login/oauth/access_token` | `api.github.com/user` + `/user/emails` | `user:email` |
| GitLab | `gitlab.com/oauth/authorize` | `gitlab.com/oauth/token` | `gitlab.com/api/v4/user` | `read_user` |

Callback URL pattern: `{BASE_URL}/auth/v1/oauth/{provider}/callback`

## 1. OAuth Flow

```
Frontend → GET /auth/v1/oauth/{provider}?redirect_uri=...
         → 302 to provider consent screen
         → User authorizes
         → Provider redirects to GET /auth/v1/oauth/{provider}/callback?code=...&state=...
         → Auth service: verify state, exchange code, fetch profile
         → Auth service: find or create user, create/update identity
         → Auth service: generate JWT + refresh token
         → 302 redirect_uri?access_token=...&refresh_token=...&expires_in=900&token_type=Bearer
```

State parameter (CSRF protection): random token stored in `garance_auth.oauth_states` with 10-minute TTL. Verified at callback, consumed (deleted) after use. Expired states cleaned up lazily at each callback.

### Token delivery security

Tokens in query params are visible in browser history and server logs. Accepted trade-off for MVP (same as Supabase). Mitigation: tokens are short-lived (15min access, 30d refresh with rotation). Future improvement: switch to fragment (`#access_token=...`) or one-time code exchange.

### Redirect URI validation

The `redirect_uri` provided by the frontend is validated against the project's `BASE_URL` origin. If `redirect_uri` doesn't start with `BASE_URL`, the request is rejected with 400. This prevents open redirect attacks. In dev mode (`BASE_URL=http://localhost:*`), any localhost origin is accepted.

## 2. User Creation / Linking

At callback, after fetching the user profile from the provider:

1. **Search `identities` by (provider, provider_user_id)**
   - Found → existing user. Update `provider_data` (name, avatar may change). Generate tokens.
   - Not found → continue to step 2.

2. **Search `users` by email**
   - Found → link provider to existing account. Create new identity. Generate tokens.
   - Not found → create new user (`encrypted_password = NULL`, `email_verified = TRUE`). Create identity. Generate tokens.

Key behaviors:
- Email/password user who later signs in via OAuth (same email) → accounts linked automatically
- OAuth-only users have `encrypted_password = NULL` — cannot sign in via password
- A user can have multiple identities (Google + GitHub linked to same account)
- `email_verified = TRUE` for OAuth users (provider already verified the email)
- **Email normalization:** all emails lowercased before lookup/insert to prevent case-mismatch duplicates
- **Banned users:** callback checks `user.BannedAt` → 403 if banned
- **No email from provider:** returns 400 "email is required" (GitHub private email case)
- **Race condition on create:** if `CreateUser` returns `ErrEmailAlreadyTaken`, retry via `GetUserByEmail` → link identity
- **Disabled/missing provider:** `GET /auth/v1/oauth/{provider}` returns 404 if not configured or `enabled = false`
- **Update identity:** returning users get `provider_data` updated (name/avatar changes). New `UpdateIdentityProviderData` store method required

## 3. Database

### Provider configuration (new table)

```sql
CREATE TABLE IF NOT EXISTS garance_platform.oauth_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT UNIQUE NOT NULL,
    client_id TEXT NOT NULL,
    client_secret_encrypted TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    scopes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### OAuth states (new table)

```sql
CREATE TABLE IF NOT EXISTS garance_auth.oauth_states (
    state TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '10 minutes'
);
```

### Encryption

`client_secret_encrypted` is AES-256-GCM encrypted using a master key from `ENCRYPTION_KEY` env var (32 bytes, base64-encoded).

- Dev mode: if `ENCRYPTION_KEY` is not set, uses `SHA-256("garance-dev-encryption-key")` as the 32-byte key. Logs a warning at startup.
- Key rotation is not supported in V1. Changing `ENCRYPTION_KEY` makes existing secrets unreadable (re-configure providers needed).
- Production: `ENCRYPTION_KEY` is required. The service refuses to start without it if `GARANCE_ENV=production`.

### Existing tables used

- `garance_auth.users` — no changes
- `garance_auth.identities` — no changes (already has provider, provider_user_id, provider_data)
- `garance_auth.sessions` — no changes (OAuth login creates a session like email/password)

## 4. Provider Configuration API

Admin endpoints for the dashboard. No JWT required — these are on the internal HTTP port (4001), not exposed via the public Gateway (8080).

| Endpoint | Method | Description |
|---|---|---|
| `/auth/v1/admin/providers` | GET | List providers (secrets masked) |
| `/auth/v1/admin/providers` | POST | Add a provider |
| `/auth/v1/admin/providers/{provider}` | PATCH | Update a provider |
| `/auth/v1/admin/providers/{provider}` | DELETE | Remove a provider |

**GET response:**
```json
[
  {
    "provider": "google",
    "client_id": "123456789.apps.googleusercontent.com",
    "has_secret": true,
    "enabled": true,
    "scopes": "email,profile",
    "callback_url": "http://localhost:8080/auth/v1/oauth/google/callback",
    "created_at": "2026-03-26T12:00:00Z"
  }
]
```

`client_secret` is NEVER returned. `has_secret: true` indicates it's configured. `callback_url` is computed from `BASE_URL` to help the developer configure the provider console.

**POST request:**
```json
{
  "provider": "google",
  "client_id": "123456789.apps.googleusercontent.com",
  "client_secret": "GOCSPX-...",
  "scopes": "email,profile"
}
```

## 5. Provider Interface

Each provider implements:

```go
type OAuthProvider interface {
    AuthorizeURL(state, redirectURI string) string
    ExchangeCode(ctx context.Context, code, redirectURI string) (*OAuthToken, error)
    GetUserProfile(ctx context.Context, accessToken string) (*OAuthProfile, error)
}

type OAuthToken struct {
    AccessToken  string
    RefreshToken string
    TokenType    string
    ExpiresIn    int
}

type OAuthProfile struct {
    ProviderUserID string
    Email          string
    Name           string
    AvatarURL      string
    Raw            map[string]interface{}
}
```

Provider implementations are in `internal/oauth/google.go`, `github.go`, `gitlab.go`. Each is ~50 lines — the OAuth2 flow is nearly identical, only the URLs and profile parsing differ.

## 6. SDK Integration

`signInWithOAuth` in `@garance/sdk` redirects the browser (not a fetch):

```typescript
signInWithOAuth(params: { provider: string; redirectUri?: string }) {
  const redirectUri = params.redirectUri || window.location.href
  window.location.href = `${baseUrl}/auth/v1/oauth/${params.provider}?redirect_uri=${encodeURIComponent(redirectUri)}`
}
```

No structural change to the SDK — just implementing the existing stub method.

## 7. Gateway Routes

The Gateway needs to proxy the OAuth routes (they involve browser redirects, so they must go through the public port 8080):

- `GET /auth/v1/oauth/{provider}` → Auth service (redirect)
- `GET /auth/v1/oauth/{provider}/callback` → Auth service (redirect)

These are added to the Auth proxy in the Gateway. They pass through as HTTP (not gRPC) since they involve redirects.

The admin endpoints (`/auth/v1/admin/*`) are NOT proxied through the Gateway — they're internal only.

## 8. Implementation Scope

| Component | Changes |
|---|---|
| Auth: new `internal/oauth/` | Provider interface + Google, GitHub, GitLab implementations |
| Auth: new `internal/crypto/encrypt.go` | AES-256-GCM encrypt/decrypt for secrets |
| Auth: new `internal/store/provider.go` | CRUD for oauth_providers table |
| Auth: new `internal/store/oauth_state.go` | CRUD for oauth_states table |
| Auth: `internal/store/identity.go` | Add `UpdateIdentityProviderData` method |
| Auth: `internal/service/auth.go` | New methods: OAuthAuthorize, OAuthCallback |
| Auth: `internal/handler/handler.go` | New routes: oauth initiate, callback, admin CRUD |
| Auth: `migrations/002_oauth.sql` | New tables |
| Auth: `internal/config/config.go` | Add ENCRYPTION_KEY |
| Gateway: `proxy/auth.go` | Add OAuth redirect routes |
| SDK: `src/auth.ts` | Implement signInWithOAuth |
| Dashboard: `auth/providers/page.tsx` | Provider configuration UI |

## 9. Testing

| Test | Component | What it verifies |
|---|---|---|
| `test_encrypt_decrypt_secret` | Auth crypto | AES-256-GCM roundtrip |
| `test_encrypt_wrong_key_fails` | Auth crypto | Wrong key → error |
| `test_create_provider` | Auth store | Insert, secret encrypted |
| `test_list_providers_hides_secret` | Auth store | GET returns no secrets |
| `test_duplicate_provider_rejected` | Auth store | Same provider → conflict |
| `test_delete_provider` | Auth store | Remove from DB |
| `test_oauth_state_create_and_verify` | Auth store | State CRUD + expiry |
| `test_oauth_google_authorize_url` | Auth oauth | Correct URL with scopes |
| `test_oauth_callback_creates_user` | Auth service | New user + identity + JWT |
| `test_oauth_callback_links_existing` | Auth service | Existing email → linked |
| `test_oauth_callback_returning_user` | Auth service | Known identity → login |
| `test_oauth_callback_invalid_state` | Auth service | Bad state → rejected |
| `test_oauth_callback_banned_user` | Auth service | Banned user → 403 |
| `test_oauth_callback_no_email` | Auth service | No email from provider → 400 |
| `test_oauth_callback_provider_disabled` | Auth service | Disabled provider → 404 |
| `test_oauth_state_expired` | Auth store | Expired state → rejected |
| `test_redirect_uri_validation` | Auth handler | Foreign origin → 400 |

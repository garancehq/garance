# Authentication

Garance provides a built-in authentication service with email/password sign-up, OAuth providers, magic links, JWT sessions, and account management.

## Overview

The Auth service (Go) handles:

- **Email/password** sign-up and sign-in with argon2id hashing
- **OAuth** providers (Google, GitHub, etc.)
- **Magic links** via email
- **JWT tokens** (access + refresh token pair)
- **Session management** and token refresh
- **Account deletion** (GDPR-compliant)

All auth endpoints are available at `/auth/v1/` through the Gateway.

## Email/password

### Sign up

```bash
curl -X POST http://localhost:8080/auth/v1/signup \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "password": "supersecret"}'
```

Response:

```json
{
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "alice@example.com",
    "email_verified": false,
    "role": "user",
    "created_at": "2026-03-26T10:00:00Z",
    "updated_at": "2026-03-26T10:00:00Z"
  },
  "token_pair": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "dGhpcyBpcyBhIHJlZnJl...",
    "expires_in": 3600,
    "token_type": "Bearer"
  }
}
```

### Sign in

```bash
curl -X POST http://localhost:8080/auth/v1/signin \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com", "password": "supersecret"}'
```

Returns the same `AuthResponse` structure with a fresh token pair.

## OAuth

Garance supports OAuth sign-in via redirect flow. Supported providers depend on your configuration.

### Initiate OAuth flow

Redirect the user to:

```
GET http://localhost:8080/auth/v1/oauth/{provider}?redirect_uri=https://myapp.com/callback
```

Where `{provider}` is one of: `google`, `github`, `gitlab`, etc.

After authentication, the user is redirected back to your `redirect_uri` with tokens as query parameters.

### SDK usage

```typescript
// Redirects the browser to the OAuth provider
await garance.auth.signInWithOAuth({
  provider: 'github',
  redirectUri: 'https://myapp.com/callback',
})
```

## Magic links

Send a magic link to the user's email for passwordless sign-in. Requires SMTP configuration.

```bash
curl -X POST http://localhost:8080/auth/v1/magic-link \
  -H "Content-Type: application/json" \
  -d '{"email": "alice@example.com"}'
```

The user receives an email with a link that signs them in automatically.

### SDK usage

```typescript
await garance.auth.signInWithMagicLink({ email: 'alice@example.com' })
```

## JWT tokens

Authentication uses a token pair:

| Token            | Lifetime  | Usage                                          |
|------------------|-----------|-------------------------------------------------|
| `access_token`   | 1 hour    | Include in `Authorization: Bearer <token>` header |
| `refresh_token`  | 30 days   | Exchange for a new token pair                   |

### Using the access token

All authenticated requests must include the access token:

```bash
curl http://localhost:8080/api/v1/posts \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

The Gateway validates the JWT and injects the user's `user_id`, `role`, and `project_id` into the downstream gRPC calls to the Engine.

### Refreshing tokens

```bash
curl -X POST http://localhost:8080/auth/v1/token/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "dGhpcyBpcyBhIHJlZnJl..."}'
```

Returns a new `AuthResponse` with fresh tokens.

## Sign out

Invalidate the refresh token:

```bash
curl -X POST http://localhost:8080/auth/v1/signout \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "dGhpcyBpcyBhIHJlZnJl..."}'
```

## User management

### Get current user

```bash
curl http://localhost:8080/auth/v1/user \
  -H "Authorization: Bearer <access_token>"
```

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "alice@example.com",
  "email_verified": false,
  "role": "user",
  "created_at": "2026-03-26T10:00:00Z",
  "updated_at": "2026-03-26T10:00:00Z"
}
```

### Delete account

```bash
curl -X DELETE http://localhost:8080/auth/v1/user \
  -H "Authorization: Bearer <access_token>"
```

This permanently deletes the user account and all associated data (GDPR-compliant).

## SDK examples

```typescript
import { createClient } from '@garance/sdk'

const garance = createClient({ url: 'http://localhost:8080' })

// Sign up -- automatically stores the access token
const { data, error } = await garance.auth.signUp({
  email: 'alice@example.com',
  password: 'supersecret',
})

// Sign in
await garance.auth.signIn({
  email: 'alice@example.com',
  password: 'supersecret',
})
// After signIn, the SDK automatically attaches the access token to all requests

// Get the current user
const { data: user } = await garance.auth.getUser()
console.log(user.email) // alice@example.com

// Refresh the token
await garance.auth.refreshToken(refreshToken)

// Sign out
await garance.auth.signOut(refreshToken)

// Delete the account
await garance.auth.deleteUser()
```

## SMTP configuration

For email verification and magic links, configure SMTP in your environment:

| Variable    | Description                        | Default                |
|-------------|------------------------------------|------------------------|
| `SMTP_HOST` | SMTP server hostname               | _(none)_               |
| `SMTP_PORT` | SMTP server port                   | `587`                  |
| `SMTP_USER` | SMTP username                      | _(none)_               |
| `SMTP_PASS` | SMTP password                      | _(none)_               |
| `SMTP_FROM` | Sender email address               | `noreply@garance.io`   |

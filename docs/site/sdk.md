# TypeScript SDK

The `@garance/sdk` package provides a typed client for interacting with your Garance backend from TypeScript or JavaScript.

## Installation

```bash
npm install @garance/sdk
```

## Create a client

```typescript
import { createClient } from '@garance/sdk'

const garance = createClient({
  url: 'http://localhost:8080',
})
```

### Options

| Option    | Type                       | Description                          |
|-----------|----------------------------|--------------------------------------|
| `url`     | `string`                   | Gateway URL (e.g. `http://localhost:8080`) |
| `headers` | `Record<string, string>`   | Custom headers to include in every request |

The client exposes three modules: `auth`, `from()` (data), and `storage`.

## Authentication

The `auth` module handles sign-up, sign-in, token management, and user operations.

### Sign up

```typescript
const { data, error } = await garance.auth.signUp({
  email: 'alice@example.com',
  password: 'supersecret',
})
// data.user — the created user
// data.token_pair — access + refresh tokens
```

After a successful sign-up, the SDK automatically stores the access token and attaches it to all subsequent requests.

### Sign in

```typescript
const { data, error } = await garance.auth.signIn({
  email: 'alice@example.com',
  password: 'supersecret',
})
```

### Magic link

```typescript
await garance.auth.signInWithMagicLink({ email: 'alice@example.com' })
```

### OAuth

```typescript
// Redirects the browser to the OAuth provider
await garance.auth.signInWithOAuth({
  provider: 'github',
  redirectUri: 'https://myapp.com/callback',
})
```

### Refresh token

```typescript
const { data } = await garance.auth.refreshToken(refreshToken)
// Automatically updates the internal access token
```

### Sign out

```typescript
await garance.auth.signOut(refreshToken)
```

### Get current user

```typescript
const { data: user } = await garance.auth.getUser()
// user.id, user.email, user.role, ...
```

### Delete account

```typescript
await garance.auth.deleteUser()
```

## Data queries

Use `garance.from(table)` to build queries against your database tables.

### Query rows

```typescript
const { data, error } = await garance
  .from('posts')
  .eq('published', true)
  .order('created_at', 'desc')
  .limit(10)
  .execute()
```

### Select specific columns

```typescript
const { data } = await garance
  .from('posts')
  .select('id,title,created_at')
  .execute()
```

### Filter operators

| Method                  | SQL equivalent        | Example                          |
|-------------------------|-----------------------|----------------------------------|
| `.eq(col, value)`       | `= value`            | `.eq('published', true)`         |
| `.neq(col, value)`      | `!= value`           | `.neq('status', 'archived')`     |
| `.gt(col, value)`       | `> value`             | `.gt('age', 18)`                 |
| `.gte(col, value)`      | `>= value`            | `.gte('price', 10)`              |
| `.lt(col, value)`       | `< value`             | `.lt('stock', 5)`                |
| `.lte(col, value)`      | `<= value`            | `.lte('rating', 3)`              |

### Ordering

```typescript
.order('created_at', 'desc')  // ORDER BY created_at DESC
.order('name', 'asc')         // ORDER BY name ASC (default)
```

### Pagination

```typescript
.limit(20)     // LIMIT 20
.offset(40)    // OFFSET 40
```

### Get a single row by ID

```typescript
const { data: post } = await garance
  .from('posts')
  .getById('550e8400-e29b-41d4-a716-446655440000')
```

### Insert a row

```typescript
const { data: post, error } = await garance
  .from('posts')
  .insert({
    title: 'Hello World',
    content: 'My first post',
    published: true,
  })
```

### Update a row

```typescript
const { data: updated } = await garance
  .from('posts')
  .update('550e8400-e29b-41d4-a716-446655440000', {
    title: 'Updated Title',
  })
```

### Delete a row

```typescript
await garance.from('posts')
  .deleteById('550e8400-e29b-41d4-a716-446655440000')
```

## Storage

The `storage` module handles file uploads, downloads, and bucket operations.

### Upload a file

```typescript
const file = new File([buffer], 'photo.jpg', { type: 'image/jpeg' })

const { data, error } = await garance.storage
  .from('avatars')
  .upload('alice/photo.jpg', file)
```

### Get a public URL

```typescript
const url = garance.storage.from('avatars').getPublicUrl('alice/photo.jpg')
```

### Get a public URL with transformations

```typescript
const url = garance.storage.from('avatars').getPublicUrl('alice/photo.jpg', {
  transform: { width: 200, height: 200, format: 'webp' },
})
```

### Create a signed URL

```typescript
const { data } = await garance.storage
  .from('documents')
  .createSignedUrl('report.pdf', { expiresIn: 3600 }) // 1 hour
```

### List files in a bucket

```typescript
const { data: files } = await garance.storage.from('avatars').list({
  prefix: 'alice/',
  limit: 20,
  offset: 0,
})
```

### Delete a file

```typescript
await garance.storage.from('avatars').remove('alice/photo.jpg')
```

## Error handling

All methods return a `{ data, error }` tuple. Check `error` before using `data`:

```typescript
const { data, error } = await garance.from('posts').execute()

if (error) {
  console.error(error.error.code)    // e.g. 'NOT_FOUND'
  console.error(error.error.message) // e.g. 'Table not found'
  console.error(error.error.status)  // e.g. 404
  return
}

// data is guaranteed to be non-null here
console.log(data)
```

### Error structure

```typescript
interface GaranceError {
  error: {
    code: string     // Machine-readable error code
    message: string  // Human-readable description
    status: number   // HTTP status code
  }
}
```

### Network errors

If the request fails at the network level (no response from the server), the error has code `NETWORK_ERROR` and status `0`.

## TypeScript types

Generate typed client types from your database schema:

```bash
garance gen types --lang ts
```

This produces `types/garance.ts` with interfaces matching your database tables. Use them with generics:

```typescript
import type { Post, User } from './types/garance'

const { data } = await garance.from<Post>('posts').execute()
//     ^? Post[]

const { data: user } = await garance.from<User>('users').getById(id)
//     ^? User
```

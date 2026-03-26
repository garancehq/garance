# REST API

All HTTP endpoints are served through the Gateway at `http://localhost:8080`.

## Data API

Base path: `/api/v1`

Endpoints are auto-generated from your database tables. If you have a `posts` table, you get `/api/v1/posts`.

### List rows

```http
GET /api/v1/{table}
```

**Query parameters:**

| Parameter     | Description                                | Example                       |
|---------------|--------------------------------------------|-------------------------------|
| `{column}`    | Filter by column with operator prefix      | `published=eq.true`           |
| `select`      | Comma-separated columns to return          | `select=id,title,created_at`  |
| `order`       | Column and direction                       | `order=created_at.desc`       |
| `limit`       | Maximum number of rows                     | `limit=20`                    |
| `offset`      | Number of rows to skip                     | `offset=40`                   |

**Filter operators:**

| Operator | Description         | Example              |
|----------|---------------------|----------------------|
| `eq`     | Equal               | `status=eq.active`   |
| `neq`    | Not equal           | `status=neq.deleted` |
| `gt`     | Greater than        | `age=gt.18`          |
| `gte`    | Greater or equal    | `price=gte.10`       |
| `lt`     | Less than           | `stock=lt.5`         |
| `lte`    | Less or equal       | `rating=lte.3`       |

**Example:**

```bash
curl "http://localhost:8080/api/v1/posts?published=eq.true&order=created_at.desc&limit=10"
```

**Response:**

```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "title": "Hello World",
    "published": true,
    "author_id": "...",
    "created_at": "2026-03-26T10:00:00Z"
  }
]
```

---

### Get a single row

```http
GET /api/v1/{table}/{id}
```

**Example:**

```bash
curl http://localhost:8080/api/v1/posts/550e8400-e29b-41d4-a716-446655440000
```

---

### Insert a row

```http
POST /api/v1/{table}
Content-Type: application/json
```

**Body:** JSON object with column values.

```bash
curl -X POST http://localhost:8080/api/v1/posts \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"title": "New post", "content": "Body text", "published": false}'
```

**Response:** The created row (HTTP 201).

---

### Update a row

```http
PATCH /api/v1/{table}/{id}
Content-Type: application/json
```

**Body:** JSON object with fields to update.

```bash
curl -X PATCH http://localhost:8080/api/v1/posts/550e8400-... \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"title": "Updated title"}'
```

**Response:** The updated row.

---

### Delete a row

```http
DELETE /api/v1/{table}/{id}
```

```bash
curl -X DELETE http://localhost:8080/api/v1/posts/550e8400-... \
  -H "Authorization: Bearer <token>"
```

**Response:** HTTP 204 No Content.

---

## Auth API

Base path: `/auth/v1`

### Sign up

```http
POST /auth/v1/signup
Content-Type: application/json

{"email": "alice@example.com", "password": "supersecret"}
```

**Response:** `AuthResponse` with `user` and `token_pair`.

---

### Sign in

```http
POST /auth/v1/signin
Content-Type: application/json

{"email": "alice@example.com", "password": "supersecret"}
```

**Response:** `AuthResponse` with `user` and `token_pair`.

---

### Magic link

```http
POST /auth/v1/magic-link
Content-Type: application/json

{"email": "alice@example.com"}
```

**Response:** HTTP 200 (email sent).

---

### OAuth

```http
GET /auth/v1/oauth/{provider}?redirect_uri=https://myapp.com/callback
```

Redirects to the OAuth provider. After auth, redirects back to `redirect_uri` with tokens.

---

### Refresh token

```http
POST /auth/v1/token/refresh
Content-Type: application/json

{"refresh_token": "..."}
```

**Response:** `AuthResponse` with new `token_pair`.

---

### Sign out

```http
POST /auth/v1/signout
Content-Type: application/json

{"refresh_token": "..."}
```

**Response:** HTTP 200.

---

### Get current user

```http
GET /auth/v1/user
Authorization: Bearer <access_token>
```

**Response:**

```json
{
  "id": "550e8400-...",
  "email": "alice@example.com",
  "email_verified": false,
  "role": "user",
  "created_at": "2026-03-26T10:00:00Z",
  "updated_at": "2026-03-26T10:00:00Z"
}
```

---

### Delete user

```http
DELETE /auth/v1/user
Authorization: Bearer <access_token>
```

**Response:** HTTP 204 No Content.

---

## Storage API

Base path: `/storage/v1`

### Upload a file

```http
POST /storage/v1/{bucket}/upload
Authorization: Bearer <access_token>
Content-Type: multipart/form-data

file: <binary>
```

**Response:**

```json
{
  "id": "f47ac10b-...",
  "bucket": "avatars",
  "name": "photo.jpg",
  "size": 245760,
  "mime_type": "image/jpeg",
  "owner_id": "550e8400-...",
  "created_at": "2026-03-26T10:00:00Z"
}
```

---

### Download a file

```http
GET /storage/v1/{bucket}/{path}
```

For public buckets, no authorization is needed. For private buckets, use a signed URL or include the Bearer token.

---

### List files

```http
GET /storage/v1/{bucket}?prefix=folder/&limit=20&offset=0
Authorization: Bearer <access_token>
```

**Response:** Array of file objects.

---

### Delete a file

```http
DELETE /storage/v1/{bucket}/{path}
Authorization: Bearer <access_token>
```

**Response:** HTTP 204 No Content.

---

### Create a signed URL

```http
POST /storage/v1/{bucket}/signed-url
Authorization: Bearer <access_token>
Content-Type: application/json

{"file_name": "report.pdf", "expires_in": 3600}
```

**Response:**

```json
{
  "signed_url": "http://localhost:8080/storage/v1/documents/report.pdf?token=abc123&expires=1711454400"
}
```

---

## Admin API

Admin endpoints for managing the Engine. These require admin-level access.

### List tables

```http
GET /api/v1/_tables
```

Returns a summary of all database tables with column counts and approximate row counts.

---

### Get schema

```http
GET /api/v1/_schema?table=posts
```

Returns the JSON schema for one or all tables.

---

### Execute SQL

```http
POST /api/v1/_sql
Content-Type: application/json

{"sql": "SELECT * FROM posts LIMIT 10", "readwrite": false}
```

**Response:**

```json
{
  "columns": ["id", "title", "published", "created_at"],
  "rows_json": "[...]",
  "row_count": 10,
  "duration_ms": 3
}
```

---

### Preview migration

```http
POST /api/v1/_migrate/preview
Content-Type: application/json

{"desired_schema": { ... }}
```

**Response:**

```json
{
  "sql": "CREATE TABLE posts (...);",
  "destructive": false,
  "has_changes": true,
  "operations": [
    {"op": "create_table", "target": "posts"}
  ]
}
```

---

### Apply migration

```http
POST /api/v1/_migrate/apply
Content-Type: application/json

{"sql": "CREATE TABLE posts (...);" }
```

**Response:** HTTP 200 on success.

---

### Reload schema

```http
POST /api/v1/_schema/reload
```

Forces the Engine to re-introspect the database. Returns the number of tables and the reload timestamp.

---

## Error format

All errors follow a consistent JSON structure:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Table 'foobar' does not exist",
    "status": 404
  }
}
```

| Field     | Type     | Description                                   |
|-----------|----------|-----------------------------------------------|
| `code`    | `string` | Machine-readable error code                   |
| `message` | `string` | Human-readable description                    |
| `status`  | `number` | HTTP status code                              |

### Common error codes

| Code                | Status | Description                          |
|---------------------|--------|--------------------------------------|
| `NOT_FOUND`         | 404    | Resource not found                   |
| `UNAUTHORIZED`      | 401    | Missing or invalid access token      |
| `FORBIDDEN`         | 403    | Insufficient permissions             |
| `VALIDATION_ERROR`  | 400    | Invalid request body or parameters   |
| `CONFLICT`          | 409    | Duplicate key or constraint violation|
| `INTERNAL_ERROR`    | 500    | Unexpected server error              |
| `NETWORK_ERROR`     | 0      | Client-side network failure (SDK)    |

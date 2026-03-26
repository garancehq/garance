# Storage

Garance provides S3-compatible file storage backed by MinIO. Define buckets in your schema with size limits, MIME type restrictions, and access rules.

## Overview

The Storage service (Go) handles:

- **Bucket management** -- create, list, and delete buckets
- **File upload/download** -- multipart upload, streaming download
- **Signed URLs** -- time-limited URLs for private files
- **Access control** -- per-bucket read/write/delete rules

All storage endpoints are available at `/storage/v1/` through the Gateway.

## Defining buckets

Buckets are defined in your `garance.schema.ts` under the `storage` key:

```typescript
import { defineSchema, bucket } from '@garance/schema'

export default defineSchema({
  // tables...

  storage: {
    avatars: bucket({
      maxFileSize: '5mb',
      allowedMimeTypes: ['image/jpeg', 'image/png', 'image/webp'],
      access: {
        read: 'public',
        write: (ctx) => ctx.isAuthenticated(),
      },
    }),

    documents: bucket({
      maxFileSize: '50mb',
      allowedMimeTypes: ['application/pdf', 'application/msword'],
      access: {
        read: (ctx) => ctx.isAuthenticated(),
        write: (ctx) => ctx.isAuthenticated(),
        delete: (ctx) => ctx.isOwner(),
      },
    }),
  },
})
```

### Bucket options

| Option             | Type       | Description                                    |
|--------------------|------------|------------------------------------------------|
| `maxFileSize`      | `string`   | Maximum file size (e.g. `'5mb'`, `'50mb'`)     |
| `allowedMimeTypes` | `string[]` | Whitelist of MIME types                         |
| `access.read`      | Rule       | `'public'` or `(ctx) => ...`                   |
| `access.write`     | Rule       | `(ctx) => ...`                                 |
| `access.delete`    | Rule       | `(ctx) => ...`                                 |

## Upload

### REST API

```bash
curl -X POST http://localhost:8080/storage/v1/avatars/upload \
  -H "Authorization: Bearer <access_token>" \
  -F "file=@photo.jpg"
```

Response:

```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "bucket": "avatars",
  "name": "photo.jpg",
  "size": 245760,
  "mime_type": "image/jpeg",
  "owner_id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2026-03-26T10:00:00Z"
}
```

### SDK

```typescript
const file = new File([buffer], 'photo.jpg', { type: 'image/jpeg' })

const { data, error } = await garance.storage
  .from('avatars')
  .upload('alice/photo.jpg', file)
```

## Download

### Public files

For buckets with `read: 'public'`, files are accessible directly:

```
GET http://localhost:8080/storage/v1/avatars/alice/photo.jpg
```

### Public URL (SDK)

```typescript
const url = garance.storage.from('avatars').getPublicUrl('alice/photo.jpg')
// http://localhost:8080/storage/v1/avatars/alice/photo.jpg
```

### Image transformations

Pass transform options to resize images on the fly:

```typescript
const url = garance.storage.from('avatars').getPublicUrl('alice/photo.jpg', {
  transform: { width: 200, height: 200, format: 'webp' },
})
// http://localhost:8080/storage/v1/avatars/alice/photo.jpg?w=200&h=200&f=webp
```

## Signed URLs

For private buckets, generate time-limited signed URLs:

### REST API

```bash
curl -X POST http://localhost:8080/storage/v1/documents/signed-url \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"file_name": "report.pdf", "expires_in": 3600}'
```

Response:

```json
{
  "signed_url": "http://localhost:8080/storage/v1/documents/report.pdf?token=abc123&expires=1711454400"
}
```

### SDK

```typescript
const { data } = await garance.storage
  .from('documents')
  .createSignedUrl('report.pdf', { expiresIn: 3600 })

console.log(data.signed_url)
```

## List files

### REST API

```bash
curl "http://localhost:8080/storage/v1/avatars?prefix=alice/&limit=20" \
  -H "Authorization: Bearer <access_token>"
```

### SDK

```typescript
const { data: files } = await garance.storage.from('avatars').list({
  prefix: 'alice/',
  limit: 20,
  offset: 0,
})
```

## Delete files

### REST API

```bash
curl -X DELETE http://localhost:8080/storage/v1/avatars/alice/photo.jpg \
  -H "Authorization: Bearer <access_token>"
```

### SDK

```typescript
await garance.storage.from('avatars').remove('alice/photo.jpg')
```

## S3 configuration

The Storage service connects to an S3-compatible backend (MinIO by default):

| Variable       | Description                      | Default        |
|----------------|----------------------------------|----------------|
| `S3_ENDPOINT`  | S3-compatible endpoint           | `minio:9000`   |
| `S3_ACCESS_KEY`| Access key ID                    | `minioadmin`   |
| `S3_SECRET_KEY`| Secret access key                | `minioadmin`   |
| `S3_BUCKET`    | Default bucket name              | `garance`      |

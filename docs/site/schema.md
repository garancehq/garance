# Schema DSL

Garance uses a declarative TypeScript DSL to define your database schema, relations, access rules, and storage buckets in a single file: `garance.schema.ts`.

## Overview

```typescript
import { defineSchema, table, column, relation, bucket } from '@garance/schema'

export default defineSchema({
  // Tables
  users: table({ ... }),
  posts: table({ ... }),

  // Storage buckets
  storage: {
    avatars: bucket({ ... }),
    documents: bucket({ ... }),
  },
})
```

The schema is compiled to a JSON format (`garance.schema.json`) that the Engine reads to generate migrations and enforce access rules.

## `defineSchema()`

The root function that wraps your entire schema definition.

```typescript
import { defineSchema } from '@garance/schema'

export default defineSchema({
  // table definitions go here as key: table({...})
  // storage buckets go under the `storage` key
})
```

The exported default must be the result of `defineSchema()`. The CLI and compiler use this to produce the JSON output.

## `table()`

Defines a database table. Each key inside `table()` is either a column, a relation, or an `access` block.

```typescript
import { table, column, relation } from '@garance/schema'

table({
  id: column.uuid().primaryKey().default('gen_random_uuid()'),
  email: column.text().unique().notNull(),
  name: column.text().notNull(),
  bio: column.text(),
  posts: relation.hasMany('posts', 'author_id'),
})
```

## Column types

All column types are created via the `column` object:

| Method              | PostgreSQL type | Description                     |
|---------------------|-----------------|---------------------------------|
| `column.uuid()`    | `uuid`          | UUID v4                         |
| `column.text()`    | `text`          | Variable-length string          |
| `column.int4()`    | `int4`          | 32-bit integer                  |
| `column.int8()`    | `int8`          | 64-bit integer                  |
| `column.float8()`  | `float8`        | Double precision float          |
| `column.bool()`    | `bool`          | Boolean                         |
| `column.boolean()` | `bool`          | Alias for `bool()`              |
| `column.timestamp()`   | `timestamp`   | Timestamp without time zone |
| `column.timestamptz()` | `timestamptz` | Timestamp with time zone    |
| `column.date()`    | `date`          | Calendar date                   |
| `column.jsonb()`   | `jsonb`         | Binary JSON                     |
| `column.json()`    | `json`          | Text JSON                       |
| `column.bytea()`   | `bytea`         | Binary data                     |
| `column.numeric()` | `numeric`       | Arbitrary precision number      |
| `column.serial()`  | `serial`        | Auto-incrementing 32-bit int    |
| `column.bigserial()` | `bigserial`   | Auto-incrementing 64-bit int    |

## Column modifiers

Chain modifiers after the column type:

```typescript
column.text().unique().notNull().default("'untitled'")
```

| Modifier                   | Description                                      |
|----------------------------|--------------------------------------------------|
| `.primaryKey()`            | Mark as primary key (also sets NOT NULL)          |
| `.unique()`                | Add a UNIQUE constraint                           |
| `.notNull()`               | Add a NOT NULL constraint                         |
| `.nullable()`              | Explicitly allow NULL (default behavior)          |
| `.default(value)`          | Set a SQL default expression (e.g. `'now()'`, `'gen_random_uuid()'`, `'false'`) |
| `.references(ref)`         | Add a foreign key (e.g. `'users.id'`)             |
| `.renamedFrom(oldName)`    | Rename a column in the next migration instead of dropping and recreating |

### Default expressions

The `.default()` modifier takes a SQL expression as a string:

```typescript
column.uuid().default('gen_random_uuid()')      // UUID generation
column.timestamptz().default('now()')            // Current timestamp
column.bool().default('false')                   // Boolean literal
column.int4().default('0')                       // Numeric literal
column.text().default("'draft'")                 // Text literal (note the single quotes)
```

### Foreign keys

Use `.references()` with the format `table.column`:

```typescript
column.uuid().references('users.id')
```

## Relations

Relations are declarative hints used by the Engine for joins and by the codegen for typed accessors. They do not create database constraints -- use `.references()` on the foreign key column for that.

```typescript
import { relation } from '@garance/schema'
```

| Method                                | Description                              |
|---------------------------------------|------------------------------------------|
| `relation.hasMany(table, foreignKey)` | One-to-many (e.g. user has many posts)   |
| `relation.hasOne(table, foreignKey)`  | One-to-one (e.g. user has one profile)   |
| `relation.belongsTo(table, foreignKey)` | Inverse of hasMany/hasOne              |

### Example

```typescript
// In the users table
posts: relation.hasMany('posts', 'author_id'),
profile: relation.hasOne('profiles', 'user_id'),

// In the posts table
author: relation.belongsTo('users', 'author_id'),
```

## Access rules

Access rules define row-level security policies. They are enforced by the Engine at query time.

```typescript
table({
  // columns...
  access: {
    read: (ctx) => ctx.where({ published: true }).or(ctx.isOwner('author_id')),
    write: (ctx) => ctx.isOwner('author_id'),
    delete: (ctx) => ctx.isOwner('author_id'),
  },
})
```

### Rule types

| Rule      | Description                                      |
|-----------|--------------------------------------------------|
| `read`    | Who can SELECT rows. Can be `'public'` for open access or a function. |
| `write`   | Who can INSERT and UPDATE rows.                  |
| `delete`  | Who can DELETE rows.                             |

### Access context methods

The `ctx` parameter provides these methods:

| Method                     | Description                                                |
|----------------------------|------------------------------------------------------------|
| `ctx.isOwner(column)`      | Current user's ID matches the value of `column`            |
| `ctx.isAuthenticated()`    | Current request has a valid JWT                            |
| `ctx.where(filters)`       | Row matches the given key/value filter conditions          |

### Combining conditions with `.or()`

The `.where()` method returns a builder that supports `.or()` to combine multiple conditions:

```typescript
// Readable if published OR if the user is the author
read: (ctx) => ctx.where({ published: true }).or(ctx.isOwner('author_id')),
```

### Public read access

Use the string `'public'` for completely open read access:

```typescript
access: {
  read: 'public',
  write: (ctx) => ctx.isAuthenticated(),
}
```

## Storage buckets

Storage buckets are defined under the `storage` key of `defineSchema()`:

```typescript
import { bucket } from '@garance/schema'

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
      allowedMimeTypes: ['application/pdf'],
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
| `allowedMimeTypes` | `string[]` | Allowed MIME types (e.g. `['image/jpeg']`)      |
| `access`           | `object`   | Access rules for `read`, `write`, `delete`      |

Bucket access rules work the same as table access rules, with `ctx.isAuthenticated()` and `ctx.isOwner()`.

## Migration pipeline

When you run `garance db migrate`, the following happens:

1. **Compile** -- `garance.schema.ts` is compiled to `garance.schema.json` via the `@garance/schema` compiler
2. **Diff** -- the JSON schema is sent to the Engine's `POST /api/v1/_migrate/preview` endpoint, which compares it against the current database state
3. **Preview** -- the Engine returns the SQL migration and a list of operations (create table, add column, alter column, rename column, etc.)
4. **Confirm** -- if the migration is destructive (drops tables or columns), the CLI prompts for confirmation (skip with `--yes`)
5. **Save** -- the SQL is saved to `migrations/<timestamp>_<description>.sql`
6. **Apply** -- the SQL is sent to `POST /api/v1/_migrate/apply` to execute against the database

### Column renames

To rename a column without losing data, use `.renamedFrom()`:

```typescript
// Before
name: column.text().notNull(),

// After -- renames `name` to `display_name`
display_name: column.text().notNull().renamedFrom('name'),
```

The Engine will generate `ALTER TABLE ... RENAME COLUMN name TO display_name` instead of dropping and recreating.

## Compiled output

The compiled JSON (`garance.schema.json`) has this structure:

```json
{
  "version": 1,
  "tables": {
    "users": {
      "columns": {
        "id": { "type": "uuid", "primary_key": true, "unique": false, "nullable": false, "default": "gen_random_uuid()" },
        "email": { "type": "text", "primary_key": false, "unique": true, "nullable": false }
      },
      "relations": {
        "posts": { "type": "hasMany", "table": "posts", "foreign_key": "author_id" }
      },
      "access": {
        "read": "public"
      }
    }
  },
  "storage": {
    "avatars": {
      "max_file_size": "5mb",
      "allowed_mime_types": ["image/jpeg", "image/png"],
      "access": { "read": "public", "write": [{ "type": "isAuthenticated" }] }
    }
  }
}
```

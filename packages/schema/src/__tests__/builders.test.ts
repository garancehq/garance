import { describe, it, expect } from 'vitest'
import { defineSchema, table, column, relation, bucket, compile } from '../index'

describe('column builder', () => {
  it('builds a uuid primary key', () => {
    const col = column.uuid().primaryKey().default('gen_random_uuid()')
    const built = col._build()
    expect(built.type).toBe('uuid')
    expect(built.primary_key).toBe(true)
    expect(built.nullable).toBe(false)
    expect(built.default).toBe('gen_random_uuid()')
  })

  it('builds a text column with constraints', () => {
    const col = column.text().unique().notNull()
    const built = col._build()
    expect(built.type).toBe('text')
    expect(built.unique).toBe(true)
    expect(built.nullable).toBe(false)
  })

  it('builds a nullable column', () => {
    const col = column.text()
    const built = col._build()
    expect(built.nullable).toBe(true)
  })

  it('builds a column with references', () => {
    const col = column.uuid().references('users.id')
    const built = col._build()
    expect(built.references).toBe('users.id')
  })

  it('supports renamedFrom hint', () => {
    const col = column.text().notNull().renamedFrom('old_name')
    const built = col._build()
    expect(built.renamed_from).toBe('old_name')
  })
})

describe('relation builder', () => {
  it('builds hasMany relation', () => {
    const rel = relation.hasMany('posts', 'author_id')
    const built = rel._build()
    expect(built.type).toBe('hasMany')
    expect(built.table).toBe('posts')
    expect(built.foreign_key).toBe('author_id')
  })

  it('builds belongsTo relation', () => {
    const rel = relation.belongsTo('users', 'user_id')
    const built = rel._build()
    expect(built.type).toBe('belongsTo')
  })
})

describe('table builder', () => {
  it('builds a table with columns and relations', () => {
    const t = table({
      id: column.uuid().primaryKey().default('gen_random_uuid()'),
      email: column.text().unique().notNull(),
      posts: relation.hasMany('posts', 'author_id'),
    })
    const built = t._build()
    expect(Object.keys(built.columns)).toEqual(['id', 'email'])
    expect(built.columns.id.type).toBe('uuid')
    expect(built.relations.posts.type).toBe('hasMany')
  })

  it('builds a table with access rules', () => {
    const t = table({
      id: column.uuid().primaryKey(),
      published: column.bool(),
      author_id: column.uuid(),
      access: {
        read: (ctx) => ctx.where({ published: true }).or(ctx.isOwner('author_id')),
        write: (ctx) => ctx.isOwner('author_id'),
      },
    })
    const built = t._build()
    expect(built.access).toBeDefined()
    expect(built.access!.read).toBeInstanceOf(Array)
    expect((built.access!.read as unknown[]).length).toBe(2)
    expect(built.access!.write).toBeInstanceOf(Array)
  })
})

describe('bucket builder', () => {
  it('builds a public bucket with constraints', () => {
    const b = bucket({
      maxFileSize: '5mb',
      allowedMimeTypes: ['image/jpeg', 'image/png'],
      access: {
        read: 'public',
        write: (ctx) => ctx.isAuthenticated(),
      },
    })
    const built = b._build()
    expect(built.max_file_size).toBe('5mb')
    expect(built.allowed_mime_types).toEqual(['image/jpeg', 'image/png'])
    expect(built.access!.read).toBe('public')
    expect(built.access!.write).toBeInstanceOf(Array)
  })
})

describe('defineSchema + compile', () => {
  it('compiles a full schema to JSON format', () => {
    const schema = defineSchema({
      users: table({
        id: column.uuid().primaryKey().default('gen_random_uuid()'),
        email: column.text().unique().notNull(),
        name: column.text().notNull(),
        created_at: column.timestamp().default('now()'),
        posts: relation.hasMany('posts', 'author_id'),
      }),
      posts: table({
        id: column.uuid().primaryKey().default('gen_random_uuid()'),
        title: column.text().notNull(),
        content: column.text(),
        author_id: column.uuid().references('users.id'),
        published: column.bool().default('false'),
        access: {
          read: (ctx) => ctx.where({ published: true }).or(ctx.isOwner('author_id')),
          write: (ctx) => ctx.isOwner('author_id'),
          delete: (ctx) => ctx.isOwner('author_id'),
        },
      }),
    })

    const result = compile(schema)

    expect(result.version).toBe(1)
    expect(Object.keys(result.tables)).toContain('users')
    expect(Object.keys(result.tables)).toContain('posts')
    expect(result.tables.users.columns.id.type).toBe('uuid')
    expect(result.tables.users.columns.id.primary_key).toBe(true)
    expect(result.tables.users.relations.posts.type).toBe('hasMany')
    expect(result.tables.posts.access).toBeDefined()
    expect(result.tables.posts.columns.author_id.references).toBe('users.id')
  })

  it('compiles schema with storage buckets', () => {
    const schema = defineSchema({
      users: table({
        id: column.uuid().primaryKey(),
      }),
      storage: {
        avatars: bucket({
          maxFileSize: '5mb',
          allowedMimeTypes: ['image/jpeg', 'image/png', 'image/webp'],
          access: { read: 'public', write: (ctx) => ctx.isAuthenticated() },
        }),
        documents: bucket({
          maxFileSize: '50mb',
          access: {
            read: (ctx) => ctx.isAuthenticated(),
            write: (ctx) => ctx.isAuthenticated(),
          },
        }),
      },
    })

    const result = compile(schema)
    expect(Object.keys(result.storage)).toEqual(['avatars', 'documents'])
    expect(result.storage.avatars.max_file_size).toBe('5mb')
    expect(result.storage.avatars.access!.read).toBe('public')
    expect(result.storage.documents.max_file_size).toBe('50mb')
  })

  it('output matches garance.schema.json format for Engine', () => {
    const schema = defineSchema({
      users: table({
        id: column.uuid().primaryKey().default('gen_random_uuid()'),
        email: column.text().unique().notNull(),
      }),
    })

    const json = JSON.stringify(compile(schema), null, 2)
    const parsed = JSON.parse(json)

    // Verify it matches the format the Rust Engine expects
    expect(parsed.version).toBe(1)
    expect(parsed.tables.users.columns.id.type).toBe('uuid')
    expect(parsed.tables.users.columns.id.primary_key).toBe(true)
    expect(parsed.tables.users.columns.id.default).toBe('gen_random_uuid()')
    expect(parsed.tables.users.columns.email.unique).toBe(true)
    expect(parsed.tables.users.columns.email.nullable).toBe(false)
  })
})

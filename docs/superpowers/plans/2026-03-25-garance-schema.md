# @garance/schema — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `@garance/schema` — a TypeScript package that provides the DSL for defining database schemas declaratively (`garance.schema.ts`). Compiles the TypeScript schema to `garance.schema.json` which the Rust Engine reads for migrations and type generation.

**Architecture:** TypeScript library with a builder-pattern API (`defineSchema`, `table`, `column`, `relation`, `bucket`). A `compile` function evaluates the schema definition and outputs a JSON representation matching the `GaranceSchema` format defined in the Engine's `json_schema.rs`.

**Tech Stack:** TypeScript 5.x, tsup (bundling), vitest (testing), tsx (runtime for schema compilation)

**Spec:** `docs/superpowers/specs/2026-03-25-garance-baas-design.md` (sections 5, 7)

---

## Task 1: Package Setup

**Files:**
- Create: `packages/schema/package.json`
- Create: `packages/schema/tsconfig.json`
- Create: `packages/schema/src/index.ts`

- [ ] **Step 1: Initialize package**

```json
// packages/schema/package.json
{
  "name": "@garance/schema",
  "version": "0.1.0",
  "description": "Declarative schema definition for Garance BaaS",
  "type": "module",
  "main": "./dist/index.cjs",
  "module": "./dist/index.js",
  "types": "./dist/index.d.ts",
  "exports": {
    ".": {
      "import": "./dist/index.js",
      "require": "./dist/index.cjs",
      "types": "./dist/index.d.ts"
    }
  },
  "bin": {
    "garance-schema": "./dist/cli.js"
  },
  "scripts": {
    "build": "tsup",
    "test": "vitest run",
    "test:watch": "vitest"
  },
  "files": ["dist"],
  "license": "Apache-2.0",
  "repository": {
    "type": "git",
    "url": "https://github.com/garancehq/garance"
  },
  "devDependencies": {
    "tsup": "^8.0.0",
    "typescript": "^5.7.0",
    "vitest": "^3.0.0"
  }
}
```

```json
// packages/schema/tsconfig.json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

```typescript
// packages/schema/src/index.ts
export { defineSchema } from './schema'
export { table } from './table'
export { column } from './column'
export { relation } from './relation'
export { bucket } from './storage'
export { compile } from './compiler'
export type { GaranceSchema, GaranceTable, GaranceColumn, GaranceRelation, GaranceAccess, AccessRule, GaranceBucket } from './types'
```

- [ ] **Step 2: Install dependencies**

```bash
cd /Users/jh3ady/Development/Projects/garance/packages/schema
npm install
```

- [ ] **Step 3: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add packages/
git commit -m ":tada: feat(schema): initialize @garance/schema TypeScript package"
```

---

## Task 2: Column & Table Builders

**Files:**
- Create: `packages/schema/src/types.ts`
- Create: `packages/schema/src/column.ts`
- Create: `packages/schema/src/relation.ts`
- Create: `packages/schema/src/table.ts`
- Create: `packages/schema/src/storage.ts`
- Create: `packages/schema/src/schema.ts`
- Create: `packages/schema/src/__tests__/builders.test.ts`

- [ ] **Step 1: Write output types**

These match the `GaranceSchema` JSON format that the Rust Engine reads.

```typescript
// packages/schema/src/types.ts

/** Root schema — output of compile() and content of garance.schema.json */
export interface GaranceSchema {
  version: number
  tables: Record<string, GaranceTable>
  storage: Record<string, GaranceBucket>
}

export interface GaranceTable {
  columns: Record<string, GaranceColumn>
  relations: Record<string, GaranceRelation>
  access?: GaranceAccess
}

export interface GaranceColumn {
  type: string
  primary_key: boolean
  unique: boolean
  nullable: boolean
  default?: string
  references?: string
}

export interface GaranceRelation {
  type: 'hasMany' | 'hasOne' | 'belongsTo'
  table: string
  foreign_key: string
}

export interface GaranceAccess {
  read?: AccessRule
  write?: AccessRule
  delete?: AccessRule
}

export type AccessRule = string | AccessCondition[]

export interface AccessCondition {
  type: 'isOwner' | 'isAuthenticated' | 'where'
  column?: string
  filters?: Record<string, unknown>
}

export interface GaranceBucket {
  max_file_size?: string
  allowed_mime_types?: string[]
  access?: GaranceAccess
}
```

- [ ] **Step 2: Write column builder**

```typescript
// packages/schema/src/column.ts
import type { GaranceColumn } from './types'

class ColumnBuilder {
  private _type: string
  private _primaryKey = false
  private _unique = false
  private _nullable = true // nullable by default in PG
  private _default?: string
  private _references?: string

  constructor(type: string) {
    this._type = type
  }

  primaryKey(): this {
    this._primaryKey = true
    this._nullable = false
    return this
  }

  unique(): this {
    this._unique = true
    return this
  }

  notNull(): this {
    this._nullable = false
    return this
  }

  nullable(): this {
    this._nullable = true
    return this
  }

  default(value: string): this {
    this._default = value
    return this
  }

  references(ref: string): this {
    this._references = ref
    return this
  }

  /** @internal */
  _build(): GaranceColumn {
    return {
      type: this._type,
      primary_key: this._primaryKey,
      unique: this._unique,
      nullable: this._nullable,
      ...(this._default !== undefined && { default: this._default }),
      ...(this._references !== undefined && { references: this._references }),
    }
  }
}

export const column = {
  uuid: () => new ColumnBuilder('uuid'),
  text: () => new ColumnBuilder('text'),
  int4: () => new ColumnBuilder('int4'),
  int8: () => new ColumnBuilder('int8'),
  float8: () => new ColumnBuilder('float8'),
  bool: () => new ColumnBuilder('bool'),
  boolean: () => new ColumnBuilder('bool'),
  timestamp: () => new ColumnBuilder('timestamp'),
  timestamptz: () => new ColumnBuilder('timestamptz'),
  date: () => new ColumnBuilder('date'),
  jsonb: () => new ColumnBuilder('jsonb'),
  json: () => new ColumnBuilder('json'),
  bytea: () => new ColumnBuilder('bytea'),
  numeric: () => new ColumnBuilder('numeric'),
  serial: () => new ColumnBuilder('serial'),
  bigserial: () => new ColumnBuilder('bigserial'),
}

export { ColumnBuilder }
```

- [ ] **Step 3: Write relation builder**

```typescript
// packages/schema/src/relation.ts
import type { GaranceRelation } from './types'

class RelationBuilder {
  private _type: GaranceRelation['type']
  private _table: string
  private _foreignKey: string

  constructor(type: GaranceRelation['type'], table: string, foreignKey: string) {
    this._type = type
    this._table = table
    this._foreignKey = foreignKey
  }

  /** @internal */
  _build(): GaranceRelation {
    return {
      type: this._type,
      table: this._table,
      foreign_key: this._foreignKey,
    }
  }
}

export const relation = {
  hasMany: (table: string, foreignKey: string) => new RelationBuilder('hasMany', table, foreignKey),
  hasOne: (table: string, foreignKey: string) => new RelationBuilder('hasOne', table, foreignKey),
  belongsTo: (table: string, foreignKey: string) => new RelationBuilder('belongsTo', table, foreignKey),
}

export { RelationBuilder }
```

- [ ] **Step 4: Write table builder**

```typescript
// packages/schema/src/table.ts
import type { GaranceTable, GaranceAccess, AccessRule } from './types'
import type { ColumnBuilder } from './column'
import type { RelationBuilder } from './relation'

interface AccessContext {
  isOwner(column: string): AccessCondition
  isAuthenticated(): AccessCondition
  where(filters: Record<string, unknown>): AccessConditionBuilder
}

interface AccessCondition {
  type: 'isOwner' | 'isAuthenticated' | 'where'
  column?: string
  filters?: Record<string, unknown>
}

class AccessConditionBuilder {
  private conditions: AccessCondition[] = []

  constructor(condition: AccessCondition) {
    this.conditions.push(condition)
  }

  or(other: AccessCondition | AccessConditionBuilder): AccessConditionBuilder {
    if (other instanceof AccessConditionBuilder) {
      this.conditions.push(...other.conditions)
    } else {
      this.conditions.push(other)
    }
    return this
  }

  /** @internal */
  _build(): AccessCondition[] {
    return this.conditions
  }
}

const accessContext: AccessContext = {
  isOwner: (column: string) => ({ type: 'isOwner', column }),
  isAuthenticated: () => ({ type: 'isAuthenticated' }),
  where: (filters: Record<string, unknown>) => new AccessConditionBuilder({ type: 'where', filters }),
}

type AccessRuleFn = (ctx: AccessContext) => AccessCondition | AccessConditionBuilder

interface TableDefinition {
  [key: string]: ColumnBuilder | RelationBuilder | TableAccess
}

interface TableAccess {
  read?: AccessRuleFn | 'public'
  write?: AccessRuleFn
  delete?: AccessRuleFn
}

function isColumnBuilder(v: unknown): v is ColumnBuilder {
  return v !== null && typeof v === 'object' && '_build' in v && '_type' in (v as Record<string, unknown>)
}

function isRelationBuilder(v: unknown): v is RelationBuilder {
  return v !== null && typeof v === 'object' && '_build' in v && '_table' in (v as Record<string, unknown>)
}

class TableBuilder {
  private definition: TableDefinition

  constructor(definition: TableDefinition) {
    this.definition = definition
  }

  /** @internal */
  _build(): GaranceTable {
    const columns: GaranceTable['columns'] = {}
    const relations: GaranceTable['relations'] = {}
    let access: GaranceAccess | undefined

    for (const [key, value] of Object.entries(this.definition)) {
      if (key === 'access') {
        access = buildAccess(value as TableAccess)
      } else if (isColumnBuilder(value)) {
        columns[key] = (value as ColumnBuilder)._build()
      } else if (isRelationBuilder(value)) {
        relations[key] = (value as RelationBuilder)._build()
      }
    }

    return { columns, relations, ...(access && { access }) }
  }
}

function buildAccess(def: TableAccess): GaranceAccess {
  const result: GaranceAccess = {}

  if (def.read !== undefined) {
    result.read = typeof def.read === 'string' ? def.read : buildAccessRule(def.read)
  }
  if (def.write !== undefined) {
    result.write = buildAccessRule(def.write)
  }
  if (def.delete !== undefined) {
    result.delete = buildAccessRule(def.delete)
  }

  return result
}

function buildAccessRule(fn: AccessRuleFn): AccessRule {
  const result = fn(accessContext)
  if (result instanceof AccessConditionBuilder) {
    return result._build()
  }
  return [result]
}

export function table(definition: TableDefinition): TableBuilder {
  return new TableBuilder(definition)
}

export { TableBuilder }
```

- [ ] **Step 5: Write storage bucket builder**

```typescript
// packages/schema/src/storage.ts
import type { GaranceBucket, GaranceAccess } from './types'

interface BucketDefinition {
  maxFileSize?: string
  allowedMimeTypes?: string[]
  access?: {
    read?: 'public' | ((ctx: { isAuthenticated(): { type: string } }) => { type: string })
    write?: (ctx: { isAuthenticated(): { type: string } }) => { type: string }
    delete?: (ctx: { isOwner(): { type: string } }) => { type: string }
  }
}

class BucketBuilder {
  private definition: BucketDefinition

  constructor(definition: BucketDefinition) {
    this.definition = definition
  }

  /** @internal */
  _build(): GaranceBucket {
    const result: GaranceBucket = {}

    if (this.definition.maxFileSize) {
      result.max_file_size = this.definition.maxFileSize
    }
    if (this.definition.allowedMimeTypes) {
      result.allowed_mime_types = this.definition.allowedMimeTypes
    }
    if (this.definition.access) {
      const access: GaranceAccess = {}
      const ctx = {
        isAuthenticated: () => ({ type: 'isAuthenticated' as const }),
        isOwner: () => ({ type: 'isOwner' as const }),
      }

      if (this.definition.access.read !== undefined) {
        access.read = typeof this.definition.access.read === 'string'
          ? this.definition.access.read
          : [this.definition.access.read(ctx)]
      }
      if (this.definition.access.write) {
        access.write = [this.definition.access.write(ctx)]
      }
      if (this.definition.access.delete) {
        access.delete = [this.definition.access.delete(ctx)]
      }
      result.access = access
    }

    return result
  }
}

export function bucket(definition: BucketDefinition): BucketBuilder {
  return new BucketBuilder(definition)
}

export { BucketBuilder }
```

- [ ] **Step 6: Write schema definition + compile**

```typescript
// packages/schema/src/schema.ts
import type { GaranceSchema } from './types'
import type { TableBuilder } from './table'
import type { BucketBuilder } from './storage'

interface SchemaDefinition {
  [key: string]: TableBuilder | StorageDefinition
}

interface StorageDefinition {
  [key: string]: BucketBuilder
}

class SchemaBuilder {
  private definition: Record<string, TableBuilder>
  private storage: Record<string, BucketBuilder>

  constructor(definition: SchemaDefinition) {
    this.definition = {}
    this.storage = {}

    for (const [key, value] of Object.entries(definition)) {
      if (key === 'storage' && typeof value === 'object' && !('_build' in value)) {
        // Storage buckets
        for (const [bucketName, bucketBuilder] of Object.entries(value as StorageDefinition)) {
          this.storage[bucketName] = bucketBuilder
        }
      } else if ('_build' in value) {
        this.definition[key] = value as TableBuilder
      }
    }
  }

  /** @internal */
  _build(): GaranceSchema {
    const tables: GaranceSchema['tables'] = {}
    for (const [name, builder] of Object.entries(this.definition)) {
      tables[name] = builder._build()
    }

    const storage: GaranceSchema['storage'] = {}
    for (const [name, builder] of Object.entries(this.storage)) {
      storage[name] = builder._build()
    }

    return { version: 1, tables, storage }
  }
}

export function defineSchema(definition: SchemaDefinition): SchemaBuilder {
  return new SchemaBuilder(definition)
}

export { SchemaBuilder }
```

- [ ] **Step 7: Write compiler**

```typescript
// packages/schema/src/compiler.ts
import type { GaranceSchema } from './types'
import type { SchemaBuilder } from './schema'

/** Compile a schema definition to the JSON format read by the Engine. */
export function compile(schema: SchemaBuilder): GaranceSchema {
  return schema._build()
}
```

- [ ] **Step 8: Write tests**

```typescript
// packages/schema/src/__tests__/builders.test.ts
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
```

- [ ] **Step 9: Add tsup config and build**

```typescript
// packages/schema/tsup.config.ts
import { defineConfig } from 'tsup'

export default defineConfig({
  entry: ['src/index.ts', 'src/cli.ts'],
  format: ['esm', 'cjs'],
  dts: true,
  clean: true,
  sourcemap: true,
})
```

Create CLI entry point stub:
```typescript
// packages/schema/src/cli.ts
#!/usr/bin/env node

/** CLI for compiling garance.schema.ts to garance.schema.json */

import { readFileSync, writeFileSync } from 'fs'
import { resolve } from 'path'

async function main() {
  const schemaPath = resolve(process.cwd(), 'garance.schema.ts')
  const outputPath = resolve(process.cwd(), 'garance.schema.json')

  // Dynamic import of the schema file (requires tsx or ts-node)
  try {
    const mod = await import(schemaPath)
    const schema = mod.default

    if (!schema || !('_build' in schema)) {
      console.error('Error: garance.schema.ts must export a defineSchema() result as default')
      process.exit(1)
    }

    const compiled = schema._build()
    writeFileSync(outputPath, JSON.stringify(compiled, null, 2))
    console.log(`✓ Schema compiled to ${outputPath}`)
  } catch (err) {
    console.error('Error compiling schema:', err)
    process.exit(1)
  }
}

main()
```

- [ ] **Step 10: Run tests**

Run: `cd /Users/jh3ady/Development/Projects/garance/packages/schema && npm test`
Expected: 9 tests pass.

- [ ] **Step 11: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add packages/
git commit -m ":sparkles: feat(schema): add @garance/schema DSL with column, table, relation, bucket builders and compiler"
```

---

## Summary

| Task | Description | Tests |
|---|---|---|
| 1 | Package setup (package.json, tsconfig) | 0 |
| 2 | Column, table, relation, bucket builders + compiler + tests | 9 |
| **Total** | | **9** |

### Not in this plan (deferred)

- Migration diff generation (compare garance.schema.json with current PG schema → SQL migrations)
- Schema validation (check for invalid references, duplicate column names, etc.)
- Schema versioning / changelog
- npm publishing

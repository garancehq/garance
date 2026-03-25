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

import type { GaranceSchema } from './types'
import type { SchemaBuilder } from './schema'

/** Compile a schema definition to the JSON format read by the Engine. */
export function compile(schema: SchemaBuilder): GaranceSchema {
  return schema._build()
}

#!/usr/bin/env node

/** CLI for compiling garance.schema.ts to garance.schema.json */

import { writeFileSync } from 'fs'
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
    console.log(`Schema compiled to ${outputPath}`)
  } catch (err) {
    console.error('Error compiling schema:', err)
    process.exit(1)
  }
}

main()

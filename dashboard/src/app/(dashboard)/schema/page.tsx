'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { RefreshCw } from 'lucide-react'
import { GATEWAY_URL } from '@/lib/garance'

interface Column {
  name: string
  data_type: string
  is_nullable: boolean
  is_primary_key: boolean
  is_unique: boolean
  has_default: boolean
  default_value: string | null
}

interface ForeignKey {
  columns: string[]
  referenced_table: string
  referenced_columns: string[]
}

interface Table {
  name: string
  schema: string
  columns: Column[]
  primary_key: string[] | null
  foreign_keys: ForeignKey[]
  indexes: { name: string; columns: string[]; is_unique: boolean }[]
}

interface Schema {
  tables: Record<string, Table>
}

export default function SchemaPage() {
  const [schema, setSchema] = useState<Schema | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchSchema = async () => {
    setLoading(true)
    try {
      const res = await fetch(`${GATEWAY_URL}/api/v1/_schema`)
      if (res.ok) {
        setSchema(await res.json())
      }
    } catch {
      // ignore
    }
    setLoading(false)
  }

  useEffect(() => {
    fetchSchema()
  }, [])

  const reload = async () => {
    await fetch(`${GATEWAY_URL}/api/v1/_reload`, { method: 'POST' })
    await fetchSchema()
  }

  const tables = schema
    ? Object.values(schema.tables).sort((a, b) => a.name.localeCompare(b.name))
    : []

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100">Schema</h2>
          <p className="text-sm text-zinc-500 mt-1">
            Database structure introspected from PostgreSQL
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={reload}>
          <RefreshCw className="h-3 w-3 mr-2" /> Reload
        </Button>
      </div>

      {loading ? (
        <p className="text-zinc-500">Loading schema...</p>
      ) : tables.length === 0 ? (
        <p className="text-zinc-500">
          No tables found. Create tables and reload the schema.
        </p>
      ) : (
        tables.map((table) => (
          <Card key={table.name} className="border-zinc-800 bg-zinc-900">
            <CardHeader>
              <CardTitle className="text-zinc-100 font-mono">{table.name}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="overflow-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-zinc-800">
                      <th className="text-left text-zinc-400 pb-2 pr-4">Column</th>
                      <th className="text-left text-zinc-400 pb-2 pr-4">Type</th>
                      <th className="text-left text-zinc-400 pb-2 pr-4">Nullable</th>
                      <th className="text-left text-zinc-400 pb-2 pr-4">Default</th>
                      <th className="text-left text-zinc-400 pb-2">Constraints</th>
                    </tr>
                  </thead>
                  <tbody>
                    {table.columns.map((col) => (
                      <tr key={col.name} className="border-b border-zinc-800/50">
                        <td className="py-2 pr-4 font-mono text-zinc-200">{col.name}</td>
                        <td className="py-2 pr-4 font-mono text-zinc-400">{col.data_type}</td>
                        <td className="py-2 pr-4 text-zinc-400">
                          {col.is_nullable ? 'yes' : 'no'}
                        </td>
                        <td className="py-2 pr-4 font-mono text-zinc-500 text-xs">
                          {col.default_value || '\u2014'}
                        </td>
                        <td className="py-2 space-x-1">
                          {col.is_primary_key && (
                            <Badge className="text-xs">PK</Badge>
                          )}
                          {col.is_unique && !col.is_primary_key && (
                            <Badge variant="secondary" className="text-xs">
                              UNIQUE
                            </Badge>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {table.foreign_keys.length > 0 && (
                <div className="mt-4 text-xs text-zinc-500">
                  <p className="font-semibold text-zinc-400 mb-1">Foreign Keys:</p>
                  {table.foreign_keys.map((fk, i) => (
                    <p key={i}>
                      {fk.columns.join(', ')} &rarr; {fk.referenced_table}(
                      {fk.referenced_columns.join(', ')})
                    </p>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        ))
      )}
    </div>
  )
}

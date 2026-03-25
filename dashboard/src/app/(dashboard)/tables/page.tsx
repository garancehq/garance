'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DataTable } from '@/components/data-table'
import { GATEWAY_URL } from '@/lib/garance'

export default function TablesPage() {
  const [tables, setTables] = useState<string[]>([])
  const [selectedTable, setSelectedTable] = useState<string | null>(null)
  const [rows, setRows] = useState<Record<string, unknown>[]>([])
  const [columns, setColumns] = useState<string[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    // In MVP, tables are discovered by trying known table names
    // A proper /api/v1/_tables endpoint would be better
    setTables(['users', 'posts'])
  }, [])

  const loadTable = async (table: string) => {
    setSelectedTable(table)
    setLoading(true)
    try {
      const res = await fetch(`${GATEWAY_URL}/api/v1/${table}?limit=50`)
      if (res.ok) {
        const data = await res.json()
        setRows(data)
        if (data.length > 0) {
          setColumns(Object.keys(data[0]))
        }
      }
    } catch {
      setRows([])
      setColumns([])
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="p-6 space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-zinc-100">Table Editor</h2>
        <p className="text-sm text-zinc-500 mt-1">Browse and edit your data</p>
      </div>

      <div className="flex gap-2">
        {tables.map((t) => (
          <Button
            key={t}
            variant={selectedTable === t ? 'default' : 'outline'}
            size="sm"
            onClick={() => loadTable(t)}
          >
            {t}
          </Button>
        ))}
      </div>

      {selectedTable && (
        <Card className="border-zinc-800 bg-zinc-900">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="text-zinc-100">{selectedTable}</CardTitle>
            <Badge variant="secondary">{rows.length} rows</Badge>
          </CardHeader>
          <CardContent>
            {loading ? (
              <p className="text-zinc-500">Loading...</p>
            ) : (
              <DataTable columns={columns} rows={rows} />
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

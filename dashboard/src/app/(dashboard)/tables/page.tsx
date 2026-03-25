'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DataTable } from '@/components/data-table'
import { RefreshCw } from 'lucide-react'
import { GATEWAY_URL } from '@/lib/garance'

interface TableInfo {
  name: string
  columns: number
  primary_key: string[] | null
  row_count: number | null
}

export default function TablesPage() {
  const [tables, setTables] = useState<TableInfo[]>([])
  const [selectedTable, setSelectedTable] = useState<string | null>(null)
  const [rows, setRows] = useState<Record<string, unknown>[]>([])
  const [columns, setColumns] = useState<string[]>([])
  const [loading, setLoading] = useState(false)

  const fetchTables = async () => {
    try {
      const res = await fetch(`${GATEWAY_URL}/api/v1/_tables`)
      if (res.ok) {
        const data = await res.json()
        setTables(data)
      }
    } catch {
      setTables([])
    }
  }

  useEffect(() => {
    fetchTables()
  }, [])

  const reloadSchema = async () => {
    try {
      await fetch(`${GATEWAY_URL}/api/v1/_reload`, { method: 'POST' })
      await fetchTables()
    } catch {
      // ignore
    }
  }

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
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100">Table Editor</h2>
          <p className="text-sm text-zinc-500 mt-1">Browse and edit your data</p>
        </div>
        <Button variant="outline" size="sm" onClick={reloadSchema}>
          <RefreshCw className="h-3 w-3 mr-2" /> Refresh
        </Button>
      </div>

      <div className="flex gap-2 flex-wrap">
        {tables.map((t) => (
          <Button
            key={t.name}
            variant={selectedTable === t.name ? 'default' : 'outline'}
            size="sm"
            onClick={() => loadTable(t.name)}
          >
            {t.name}
            {t.row_count !== null && (
              <Badge variant="secondary" className="ml-2 text-xs">
                {t.row_count}
              </Badge>
            )}
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

'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { DataTable } from '@/components/data-table'
import { Play } from 'lucide-react'
import { GATEWAY_URL } from '@/lib/garance'

export default function SQLEditorPage() {
  const [query, setQuery] = useState('SELECT * FROM users LIMIT 10;')
  const [results, setResults] = useState<Record<string, unknown>[]>([])
  const [columns, setColumns] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [duration, setDuration] = useState<number | null>(null)

  const executeQuery = async () => {
    setLoading(true)
    setError(null)
    const start = Date.now()

    try {
      const res = await fetch(`${GATEWAY_URL}/api/v1/rpc/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sql: query }),
      })

      setDuration(Date.now() - start)

      if (res.ok) {
        const data = await res.json()
        if (Array.isArray(data) && data.length > 0) {
          setResults(data)
          setColumns(Object.keys(data[0]))
        } else {
          setResults([])
          setColumns([])
        }
      } else {
        const err = await res.json()
        setError(err.error?.message || 'Query failed')
        setResults([])
        setColumns([])
      }
    } catch (err) {
      setError(String(err))
      setDuration(Date.now() - start)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="p-6 space-y-4 h-full flex flex-col">
      <div>
        <h2 className="text-2xl font-semibold text-zinc-100">SQL Editor</h2>
        <p className="text-sm text-zinc-500 mt-1">Execute SQL queries directly</p>
      </div>

      <div className="flex-none">
        <Textarea
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="font-mono text-sm bg-zinc-900 border-zinc-800 text-zinc-100 min-h-[120px] resize-y"
          placeholder="SELECT * FROM ..."
          onKeyDown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
              executeQuery()
            }
          }}
        />
        <div className="flex items-center gap-3 mt-3">
          <Button onClick={executeQuery} disabled={loading} size="sm">
            <Play className="h-3 w-3 mr-2" />
            {loading ? 'Executing...' : 'Run'}
          </Button>
          <span className="text-xs text-zinc-500">Ctrl+Enter to execute</span>
          {duration !== null && (
            <Badge variant="secondary" className="text-xs">{duration}ms</Badge>
          )}
        </div>
      </div>

      {error && (
        <Card className="border-red-900/50 bg-red-950/20">
          <CardContent className="p-4">
            <p className="text-red-400 text-sm font-mono">{error}</p>
          </CardContent>
        </Card>
      )}

      {results.length > 0 && (
        <Card className="flex-1 border-zinc-800 bg-zinc-900 overflow-auto">
          <CardHeader className="py-3">
            <CardTitle className="text-sm text-zinc-400">{results.length} rows</CardTitle>
          </CardHeader>
          <CardContent>
            <DataTable columns={columns} rows={results} />
          </CardContent>
        </Card>
      )}
    </div>
  )
}

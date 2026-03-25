'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Plus } from 'lucide-react'

const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080'

interface Bucket {
  id: string
  name: string
  is_public: boolean
  max_file_size: number
  allowed_mime_types: string[]
  created_at: string
}

export default function StorageBucketsPage() {
  const [buckets, setBuckets] = useState<Bucket[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetchBuckets()
  }, [])

  const fetchBuckets = async () => {
    try {
      const res = await fetch(`${GATEWAY_URL}/storage/v1/buckets`, {
        headers: { 'Authorization': `Bearer ${getToken()}` },
      })
      if (res.ok) {
        const data = await res.json()
        setBuckets(data || [])
      }
    } catch {
      // Gateway not running
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100">Buckets</h2>
          <p className="text-sm text-zinc-500 mt-1">Manage storage buckets</p>
        </div>
        <Button size="sm"><Plus className="h-3 w-3 mr-2" /> New Bucket</Button>
      </div>

      <Card className="border-zinc-800 bg-zinc-900">
        <CardContent className="pt-6">
          {loading ? (
            <p className="text-zinc-500">Loading...</p>
          ) : buckets.length === 0 ? (
            <p className="text-zinc-500 text-sm">No buckets yet. Create one to start storing files.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="border-zinc-800">
                  <TableHead className="text-zinc-400">Name</TableHead>
                  <TableHead className="text-zinc-400">Visibility</TableHead>
                  <TableHead className="text-zinc-400">Max Size</TableHead>
                  <TableHead className="text-zinc-400">Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {buckets.map((b) => (
                  <TableRow key={b.id} className="border-zinc-800">
                    <TableCell className="text-zinc-300 font-mono">{b.name}</TableCell>
                    <TableCell>
                      <Badge variant={b.is_public ? 'default' : 'secondary'}>
                        {b.is_public ? 'Public' : 'Private'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-zinc-500 text-sm">
                      {b.max_file_size ? formatBytes(b.max_file_size) : 'No limit'}
                    </TableCell>
                    <TableCell className="text-zinc-500 text-sm">
                      {new Date(b.created_at).toLocaleDateString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function getToken(): string {
  return typeof window !== 'undefined' ? localStorage.getItem('garance_token') || '' : ''
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

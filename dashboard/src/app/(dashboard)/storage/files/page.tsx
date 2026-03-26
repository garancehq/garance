'use client'

import { useEffect, useState, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'

const STORAGE_URL = process.env.NEXT_PUBLIC_STORAGE_URL || 'http://localhost:4002'

interface Bucket {
  id: string
  name: string
  is_public: boolean
  created_at: string
}

interface StorageFile {
  id: string
  bucket_id: string
  name: string
  size: number
  mime_type: string
  owner_id: string | null
  created_at: string
  updated_at: string
}

export default function StorageFilesPage() {
  const [buckets, setBuckets] = useState<Bucket[]>([])
  const [selectedBucket, setSelectedBucket] = useState<string | null>(null)
  const [files, setFiles] = useState<StorageFile[]>([])
  const [loading, setLoading] = useState(true)
  const [filesLoading, setFilesLoading] = useState(false)

  const fetchBuckets = useCallback(async () => {
    try {
      const res = await fetch(`${STORAGE_URL}/storage/v1/admin/buckets`)
      if (res.ok) {
        const data: Bucket[] = await res.json()
        setBuckets(data || [])
        if (data.length > 0 && !selectedBucket) {
          setSelectedBucket(data[0].name)
        }
      }
    } catch {
      // Storage service not running
    } finally {
      setLoading(false)
    }
  }, [selectedBucket])

  const fetchFiles = useCallback(async (bucketName: string) => {
    setFilesLoading(true)
    try {
      const res = await fetch(`${STORAGE_URL}/storage/v1/admin/buckets/${bucketName}/files?limit=50`)
      if (res.ok) {
        const data: StorageFile[] = await res.json()
        setFiles(data || [])
      }
    } catch {
      // Storage service not running
    } finally {
      setFilesLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchBuckets()
  }, [fetchBuckets])

  useEffect(() => {
    if (selectedBucket) {
      fetchFiles(selectedBucket)
    }
  }, [selectedBucket, fetchFiles])

  return (
    <div className="p-6 space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-zinc-100">Files</h2>
        <p className="text-sm text-zinc-500 mt-1">Browse and manage stored files</p>
      </div>

      {loading ? (
        <p className="text-zinc-500">Loading...</p>
      ) : buckets.length === 0 ? (
        <Card className="border-zinc-800 bg-zinc-900">
          <CardContent className="pt-6">
            <p className="text-zinc-500 text-sm">No buckets found. Create a bucket first to start storing files.</p>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="flex gap-2">
            {buckets.map((b) => (
              <button
                key={b.id}
                onClick={() => setSelectedBucket(b.name)}
                className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                  selectedBucket === b.name
                    ? 'bg-zinc-100 text-zinc-900'
                    : 'bg-zinc-800 text-zinc-400 hover:text-zinc-300'
                }`}
              >
                {b.name}
                {b.is_public && (
                  <Badge variant="secondary" className="ml-2 text-[10px] px-1 py-0">public</Badge>
                )}
              </button>
            ))}
          </div>

          <Card className="border-zinc-800 bg-zinc-900">
            <CardHeader>
              <CardTitle className="text-sm text-zinc-400">
                {selectedBucket ? `${files.length} files in ${selectedBucket}` : 'Select a bucket'}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {filesLoading ? (
                <p className="text-zinc-500">Loading files...</p>
              ) : files.length === 0 ? (
                <p className="text-zinc-500 text-sm">No files in this bucket yet.</p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow className="border-zinc-800">
                      <TableHead className="text-zinc-400">Name</TableHead>
                      <TableHead className="text-zinc-400">Size</TableHead>
                      <TableHead className="text-zinc-400">Type</TableHead>
                      <TableHead className="text-zinc-400">Created</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {files.map((file) => (
                      <TableRow key={file.id} className="border-zinc-800">
                        <TableCell className="text-zinc-300 font-mono text-sm">{file.name}</TableCell>
                        <TableCell className="text-zinc-500 text-sm">{formatBytes(file.size)}</TableCell>
                        <TableCell>
                          <Badge variant="secondary" className="font-mono text-xs">{file.mime_type}</Badge>
                        </TableCell>
                        <TableCell className="text-zinc-500 text-sm">
                          {new Date(file.created_at).toLocaleDateString()}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB'
}

'use client'

import { useEffect, useState, useCallback } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter, DialogTrigger,
} from '@/components/ui/dialog'
import { Plus, Trash2 } from 'lucide-react'

const AUTH_URL = process.env.NEXT_PUBLIC_AUTH_URL || 'http://localhost:4001'
const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080'

interface Provider {
  id: string
  provider: string
  client_id: string
  has_secret: boolean
  enabled: boolean
  scopes: string
  created_at: string
  updated_at: string
}

const SUPPORTED_PROVIDERS = ['google', 'github', 'gitlab'] as const

export default function AuthProvidersPage() {
  const [providers, setProviders] = useState<Provider[]>([])
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)

  // Form state
  const [formProvider, setFormProvider] = useState<string>('google')
  const [formClientId, setFormClientId] = useState('')
  const [formClientSecret, setFormClientSecret] = useState('')
  const [formScopes, setFormScopes] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchProviders = useCallback(async () => {
    try {
      const res = await fetch(`${AUTH_URL}/auth/v1/admin/providers`)
      if (res.ok) {
        const data = await res.json()
        setProviders(data || [])
      }
    } catch {
      // Auth service not running
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchProviders()
  }, [fetchProviders])

  const handleCreate = async () => {
    setError(null)
    setSubmitting(true)
    try {
      const res = await fetch(`${AUTH_URL}/auth/v1/admin/providers`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider: formProvider,
          client_id: formClientId,
          client_secret: formClientSecret,
          scopes: formScopes,
        }),
      })
      if (!res.ok) {
        const body = await res.json()
        setError(body?.error?.message || 'Failed to create provider')
        return
      }
      setDialogOpen(false)
      resetForm()
      fetchProviders()
    } catch {
      setError('Failed to reach Auth service')
    } finally {
      setSubmitting(false)
    }
  }

  const handleToggle = async (provider: string, enabled: boolean) => {
    try {
      await fetch(`${AUTH_URL}/auth/v1/admin/providers/${provider}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
      })
      fetchProviders()
    } catch {
      // ignore
    }
  }

  const handleDelete = async (provider: string) => {
    try {
      await fetch(`${AUTH_URL}/auth/v1/admin/providers/${provider}`, {
        method: 'DELETE',
      })
      fetchProviders()
    } catch {
      // ignore
    }
  }

  const resetForm = () => {
    setFormProvider('google')
    setFormClientId('')
    setFormClientSecret('')
    setFormScopes('')
    setError(null)
  }

  const callbackUrl = (provider: string) =>
    `${GATEWAY_URL}/auth/v1/oauth/${provider}/callback`

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-zinc-100">Providers</h2>
          <p className="text-sm text-zinc-500 mt-1">Configure OAuth authentication providers</p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogTrigger
            render={
              <Button size="sm" onClick={() => { resetForm(); setDialogOpen(true) }} />
            }
          >
            <Plus className="h-3 w-3 mr-2" /> Add Provider
          </DialogTrigger>
          <DialogContent className="sm:max-w-md bg-zinc-900 border-zinc-800">
            <DialogHeader>
              <DialogTitle className="text-zinc-100">Add OAuth Provider</DialogTitle>
              <DialogDescription>
                Configure a new OAuth provider for user authentication.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-2">
              <div className="space-y-2">
                <Label className="text-zinc-400">Provider</Label>
                <select
                  value={formProvider}
                  onChange={(e) => setFormProvider(e.target.value)}
                  className="flex h-8 w-full rounded-lg border border-input bg-zinc-950 px-2.5 py-1 text-sm text-zinc-100 outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                >
                  {SUPPORTED_PROVIDERS.map((p) => (
                    <option key={p} value={p}>
                      {p.charAt(0).toUpperCase() + p.slice(1)}
                    </option>
                  ))}
                </select>
              </div>
              <div className="space-y-2">
                <Label className="text-zinc-400">Client ID</Label>
                <Input
                  value={formClientId}
                  onChange={(e) => setFormClientId(e.target.value)}
                  placeholder="your-client-id"
                  className="bg-zinc-950 border-zinc-800"
                />
              </div>
              <div className="space-y-2">
                <Label className="text-zinc-400">Client Secret</Label>
                <Input
                  type="password"
                  value={formClientSecret}
                  onChange={(e) => setFormClientSecret(e.target.value)}
                  placeholder="your-client-secret"
                  className="bg-zinc-950 border-zinc-800"
                />
              </div>
              <div className="space-y-2">
                <Label className="text-zinc-400">Scopes</Label>
                <Input
                  value={formScopes}
                  onChange={(e) => setFormScopes(e.target.value)}
                  placeholder="e.g. email,profile"
                  className="bg-zinc-950 border-zinc-800"
                />
              </div>
              {error && (
                <p className="text-sm text-red-400">{error}</p>
              )}
            </div>
            <DialogFooter>
              <Button
                size="sm"
                onClick={handleCreate}
                disabled={submitting || !formClientId || !formClientSecret}
              >
                {submitting ? 'Creating...' : 'Create Provider'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <Card className="border-zinc-800 bg-zinc-900">
        <CardContent className="pt-6">
          {loading ? (
            <p className="text-zinc-500">Loading...</p>
          ) : providers.length === 0 ? (
            <p className="text-zinc-500 text-sm">No providers configured. Add one to enable OAuth sign-in.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="border-zinc-800">
                  <TableHead className="text-zinc-400">Provider</TableHead>
                  <TableHead className="text-zinc-400">Client ID</TableHead>
                  <TableHead className="text-zinc-400">Status</TableHead>
                  <TableHead className="text-zinc-400">Callback URL</TableHead>
                  <TableHead className="text-zinc-400 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {providers.map((p) => (
                  <TableRow key={p.id} className="border-zinc-800">
                    <TableCell className="text-zinc-300 font-medium">
                      {p.provider.charAt(0).toUpperCase() + p.provider.slice(1)}
                    </TableCell>
                    <TableCell className="text-zinc-400 font-mono text-sm">
                      {p.client_id}
                    </TableCell>
                    <TableCell>
                      <button
                        onClick={() => handleToggle(p.provider, !p.enabled)}
                        className="cursor-pointer"
                      >
                        <Badge variant={p.enabled ? 'default' : 'secondary'}>
                          {p.enabled ? 'Enabled' : 'Disabled'}
                        </Badge>
                      </button>
                    </TableCell>
                    <TableCell className="text-zinc-500 font-mono text-xs max-w-[300px] truncate">
                      {callbackUrl(p.provider)}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="destructive"
                        size="icon-xs"
                        onClick={() => handleDelete(p.provider)}
                      >
                        <Trash2 className="h-3 w-3" />
                      </Button>
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

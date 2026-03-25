# Garance Dashboard — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Garance Dashboard — a Next.js admin interface for managing the BaaS. Provides a table editor, SQL editor, user management, storage browser, and settings. Consumes the Gateway API via `@garance/sdk`.

**Architecture:** Next.js 15 App Router with shadcn/ui + Tailwind CSS. Dark mode by default, Geist font. Server components for data fetching, client components for interactivity. The dashboard runs on port 3000 and talks to the Gateway on port 8080.

**Tech Stack:** Next.js 15, React 19, shadcn/ui, Tailwind CSS v4, Geist font, @garance/sdk (local workspace)

**Spec:** `docs/superpowers/specs/2026-03-25-garance-baas-design.md` (section 8)

---

## Task 1: Next.js Project Setup

**Files:**
- Create: `dashboard/` (via create-next-app)
- Initialize shadcn/ui
- Configure dark mode, Geist font

- [ ] **Step 1: Create Next.js app**

```bash
cd /Users/jh3ady/Development/Projects/garance
npx create-next-app@latest dashboard --typescript --tailwind --eslint --app --src-dir --import-alias "@/*" --no-turbopack
```

- [ ] **Step 2: Install shadcn/ui**

```bash
cd /Users/jh3ady/Development/Projects/garance/dashboard
npx shadcn@latest init -d
```

- [ ] **Step 3: Install core shadcn components**

```bash
npx shadcn@latest add button card input label table tabs badge dialog sheet separator scroll-area dropdown-menu toast textarea command
```

- [ ] **Step 4: Configure dark mode as default**

In `dashboard/src/app/layout.tsx`, add `className="dark"` to the `<html>` tag.

- [ ] **Step 5: Create SDK client wrapper**

```typescript
// dashboard/src/lib/garance.ts
import { createClient } from '@garance/sdk'

const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080'

export const garance = createClient({ url: GATEWAY_URL })
```

Install the SDK as a workspace dependency or copy it:
```bash
cd /Users/jh3ady/Development/Projects/garance/dashboard
npm install ../sdks/typescript
```

- [ ] **Step 6: Verify dev server starts**

```bash
cd /Users/jh3ady/Development/Projects/garance/dashboard && npm run dev &
# Wait a few seconds, then check
curl -s http://localhost:3000 | head -5
# Kill dev server
kill %1
```

- [ ] **Step 7: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add dashboard/
git commit -m ":tada: feat(dashboard): initialize Next.js with shadcn/ui and dark mode"
```

---

## Task 2: Layout & Navigation

**Files:**
- Create: `dashboard/src/components/sidebar.tsx`
- Create: `dashboard/src/components/header.tsx`
- Modify: `dashboard/src/app/layout.tsx`
- Create: `dashboard/src/app/(dashboard)/layout.tsx`

- [ ] **Step 1: Write sidebar navigation**

```tsx
// dashboard/src/components/sidebar.tsx
'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cn } from '@/lib/utils'
import {
  Database,
  Users,
  HardDrive,
  Settings,
  Terminal,
  FileCode,
  Activity,
  Shield,
} from 'lucide-react'

const navigation = [
  { name: 'Table Editor', href: '/tables', icon: Database },
  { name: 'SQL Editor', href: '/sql', icon: Terminal },
  { name: 'Schema', href: '/schema', icon: FileCode },
  { name: 'Auth', href: '/auth/users', icon: Shield, children: [
    { name: 'Users', href: '/auth/users' },
    { name: 'Providers', href: '/auth/providers' },
  ]},
  { name: 'Storage', href: '/storage/buckets', icon: HardDrive, children: [
    { name: 'Buckets', href: '/storage/buckets' },
    { name: 'Files', href: '/storage/files' },
  ]},
  { name: 'Logs', href: '/logs', icon: Activity },
  { name: 'Settings', href: '/settings', icon: Settings },
]

export function Sidebar() {
  const pathname = usePathname()

  return (
    <aside className="w-64 border-r border-zinc-800 bg-zinc-950 flex flex-col">
      <div className="p-4 border-b border-zinc-800">
        <h1 className="text-lg font-semibold text-zinc-100 font-mono">garance</h1>
        <p className="text-xs text-zinc-500 mt-1">BaaS souverain</p>
      </div>
      <nav className="flex-1 p-3 space-y-1">
        {navigation.map((item) => {
          const isActive = pathname.startsWith(item.href)
          const Icon = item.icon
          return (
            <div key={item.name}>
              <Link
                href={item.href}
                className={cn(
                  'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
                  isActive
                    ? 'bg-zinc-800 text-zinc-100'
                    : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-900'
                )}
              >
                <Icon className="h-4 w-4" />
                {item.name}
              </Link>
              {item.children && isActive && (
                <div className="ml-7 mt-1 space-y-1">
                  {item.children.map((child) => (
                    <Link
                      key={child.href}
                      href={child.href}
                      className={cn(
                        'block px-3 py-1.5 rounded-md text-sm transition-colors',
                        pathname === child.href
                          ? 'text-zinc-100 bg-zinc-800/50'
                          : 'text-zinc-500 hover:text-zinc-300'
                      )}
                    >
                      {child.name}
                    </Link>
                  ))}
                </div>
              )}
            </div>
          )
        })}
      </nav>
    </aside>
  )
}
```

- [ ] **Step 2: Write dashboard layout**

```tsx
// dashboard/src/app/(dashboard)/layout.tsx
import { Sidebar } from '@/components/sidebar'

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen bg-zinc-950">
      <Sidebar />
      <main className="flex-1 overflow-auto">
        {children}
      </main>
    </div>
  )
}
```

- [ ] **Step 3: Install lucide-react**

```bash
cd /Users/jh3ady/Development/Projects/garance/dashboard
npm install lucide-react
```

- [ ] **Step 4: Create placeholder pages**

Create stub pages for each route:

```tsx
// dashboard/src/app/(dashboard)/tables/page.tsx
export default function TablesPage() {
  return (
    <div className="p-6">
      <h2 className="text-2xl font-semibold text-zinc-100 mb-4">Table Editor</h2>
      <p className="text-zinc-400">Select a table to view and edit data.</p>
    </div>
  )
}
```

Create similar stubs for:
- `src/app/(dashboard)/sql/page.tsx` — SQL Editor
- `src/app/(dashboard)/schema/page.tsx` — Schema
- `src/app/(dashboard)/auth/users/page.tsx` — Users
- `src/app/(dashboard)/auth/providers/page.tsx` — Providers
- `src/app/(dashboard)/storage/buckets/page.tsx` — Buckets
- `src/app/(dashboard)/storage/files/page.tsx` — Files
- `src/app/(dashboard)/logs/page.tsx` — Logs
- `src/app/(dashboard)/settings/page.tsx` — Settings

Each follows the same pattern with the page name and a short description.

- [ ] **Step 5: Create root redirect**

```tsx
// dashboard/src/app/(dashboard)/page.tsx
import { redirect } from 'next/navigation'

export default function DashboardHome() {
  redirect('/tables')
}
```

- [ ] **Step 6: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add dashboard/
git commit -m ":sparkles: feat(dashboard): add sidebar navigation and page stubs"
```

---

## Task 3: Table Editor Page

**Files:**
- Create: `dashboard/src/app/(dashboard)/tables/page.tsx` (rewrite)
- Create: `dashboard/src/components/table-editor.tsx`
- Create: `dashboard/src/components/data-table.tsx`

- [ ] **Step 1: Write table selector + data table**

```tsx
// dashboard/src/app/(dashboard)/tables/page.tsx
'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DataTable } from '@/components/data-table'

const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080'

export default function TablesPage() {
  const [tables, setTables] = useState<string[]>([])
  const [selectedTable, setSelectedTable] = useState<string | null>(null)
  const [rows, setRows] = useState<Record<string, unknown>[]>([])
  const [columns, setColumns] = useState<string[]>([])
  const [loading, setLoading] = useState(false)

  // Fetch tables from engine introspection (via health or a tables endpoint)
  // For MVP, we'll use the API to list rows and infer tables
  useEffect(() => {
    // In MVP, tables are discovered by trying known table names
    // A proper /api/v1/_tables endpoint would be better
    setTables(['users', 'posts']) // placeholder
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
```

- [ ] **Step 2: Write data table component**

```tsx
// dashboard/src/components/data-table.tsx
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

interface DataTableProps {
  columns: string[]
  rows: Record<string, unknown>[]
}

export function DataTable({ columns, rows }: DataTableProps) {
  if (columns.length === 0) {
    return <p className="text-zinc-500 text-sm">No data</p>
  }

  return (
    <div className="rounded-md border border-zinc-800 overflow-auto max-h-[600px]">
      <Table>
        <TableHeader>
          <TableRow className="border-zinc-800 hover:bg-transparent">
            {columns.map((col) => (
              <TableHead key={col} className="text-zinc-400 font-mono text-xs">
                {col}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, i) => (
            <TableRow key={i} className="border-zinc-800">
              {columns.map((col) => (
                <TableCell key={col} className="text-zinc-300 font-mono text-xs py-2">
                  {formatValue(row[col])}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return 'NULL'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}
```

- [ ] **Step 3: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add dashboard/
git commit -m ":sparkles: feat(dashboard): add table editor with data table component"
```

---

## Task 4: SQL Editor Page

**Files:**
- Modify: `dashboard/src/app/(dashboard)/sql/page.tsx`

- [ ] **Step 1: Write SQL editor page**

```tsx
// dashboard/src/app/(dashboard)/sql/page.tsx
'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { DataTable } from '@/components/data-table'
import { Play } from 'lucide-react'

const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080'

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
      // For MVP, we use the engine's direct HTTP API
      // A proper SQL execution endpoint would be better
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
```

Note: The SQL execution endpoint (`/api/v1/rpc/query`) doesn't exist yet. This page will show an error until the RPC endpoint is built. The UI is functional and ready.

- [ ] **Step 2: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add dashboard/
git commit -m ":sparkles: feat(dashboard): add SQL editor page with query execution"
```

---

## Task 5: Auth Users & Storage Pages

**Files:**
- Modify: `dashboard/src/app/(dashboard)/auth/users/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/storage/buckets/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/storage/files/page.tsx`

- [ ] **Step 1: Write auth users page**

```tsx
// dashboard/src/app/(dashboard)/auth/users/page.tsx
'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'

const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080'

interface AuthUser {
  id: string
  email: string
  email_verified: boolean
  role: string
  created_at: string
}

export default function AuthUsersPage() {
  const [users, setUsers] = useState<AuthUser[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    // In MVP, this would call an admin endpoint to list auth users
    // For now, placeholder
    setLoading(false)
    setUsers([])
  }, [])

  return (
    <div className="p-6 space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-zinc-100">Users</h2>
        <p className="text-sm text-zinc-500 mt-1">Manage authenticated users</p>
      </div>

      <Card className="border-zinc-800 bg-zinc-900">
        <CardHeader>
          <CardTitle className="text-sm text-zinc-400">
            {users.length} users
          </CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p className="text-zinc-500">Loading...</p>
          ) : users.length === 0 ? (
            <p className="text-zinc-500 text-sm">No users yet. Users will appear here after they sign up via the API.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="border-zinc-800">
                  <TableHead className="text-zinc-400">Email</TableHead>
                  <TableHead className="text-zinc-400">Role</TableHead>
                  <TableHead className="text-zinc-400">Verified</TableHead>
                  <TableHead className="text-zinc-400">Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => (
                  <TableRow key={user.id} className="border-zinc-800">
                    <TableCell className="text-zinc-300 font-mono text-sm">{user.email}</TableCell>
                    <TableCell><Badge variant="secondary">{user.role}</Badge></TableCell>
                    <TableCell>
                      <Badge variant={user.email_verified ? 'default' : 'destructive'}>
                        {user.email_verified ? 'Yes' : 'No'}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-zinc-500 text-sm">
                      {new Date(user.created_at).toLocaleDateString()}
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
```

- [ ] **Step 2: Write storage buckets page**

```tsx
// dashboard/src/app/(dashboard)/storage/buckets/page.tsx
'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
  // In MVP, token management is basic
  return typeof window !== 'undefined' ? localStorage.getItem('garance_token') || '' : ''
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}
```

- [ ] **Step 3: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add dashboard/
git commit -m ":sparkles: feat(dashboard): add auth users, storage buckets pages"
```

---

## Task 6: Settings Page & Dockerfile

**Files:**
- Modify: `dashboard/src/app/(dashboard)/settings/page.tsx`
- Create: `dashboard/Dockerfile`

- [ ] **Step 1: Write settings page**

```tsx
// dashboard/src/app/(dashboard)/settings/page.tsx
'use client'

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

export default function SettingsPage() {
  return (
    <div className="p-6 space-y-6 max-w-2xl">
      <div>
        <h2 className="text-2xl font-semibold text-zinc-100">Settings</h2>
        <p className="text-sm text-zinc-500 mt-1">Project configuration</p>
      </div>

      <Card className="border-zinc-800 bg-zinc-900">
        <CardHeader>
          <CardTitle className="text-zinc-100">API Configuration</CardTitle>
          <CardDescription>Your project's connection details</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label className="text-zinc-400">Gateway URL</Label>
            <Input
              value={process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080'}
              readOnly
              className="font-mono text-sm bg-zinc-950 border-zinc-800"
            />
          </div>
          <div className="space-y-2">
            <Label className="text-zinc-400">Project ID</Label>
            <Input
              value="local-dev"
              readOnly
              className="font-mono text-sm bg-zinc-950 border-zinc-800"
            />
          </div>
        </CardContent>
      </Card>

      <Separator className="bg-zinc-800" />

      <Card className="border-red-900/30 bg-zinc-900">
        <CardHeader>
          <CardTitle className="text-red-400">Danger Zone</CardTitle>
          <CardDescription>Irreversible actions</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="destructive" size="sm">
            Reset Database
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
```

- [ ] **Step 2: Write Dockerfile**

```dockerfile
# dashboard/Dockerfile
FROM node:22-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ENV NEXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine
WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public

ENV PORT=3000
EXPOSE 3000
CMD ["node", "server.js"]
```

Add to `dashboard/next.config.ts`:
```typescript
output: 'standalone',
```

- [ ] **Step 3: Commit**

```bash
cd /Users/jh3ady/Development/Projects/garance
git add dashboard/
git commit -m ":sparkles: feat(dashboard): add settings page and Dockerfile"
```

---

## Summary

| Task | Description | Key Pages |
|---|---|---|
| 1 | Next.js + shadcn/ui + dark mode | Setup |
| 2 | Sidebar navigation + page stubs | Layout, 9 stub pages |
| 3 | Table Editor | Data browsing with DataTable component |
| 4 | SQL Editor | Query input + results display |
| 5 | Auth Users + Storage Buckets | User list, bucket management |
| 6 | Settings + Dockerfile | Config display, danger zone, Docker |

### Not in this plan (deferred)

- Inline row editing in Table Editor
- SQL query history and snippets
- Schema visualization and diff
- File browser with preview
- OAuth provider configuration UI
- Logs page with real-time streaming
- Bucket creation dialog
- User ban/unban actions
- API docs page (OpenAPI viewer)
- Authentication for dashboard access

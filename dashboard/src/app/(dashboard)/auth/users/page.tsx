'use client'

import { useEffect, useState, useCallback } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'

const AUTH_URL = process.env.NEXT_PUBLIC_AUTH_URL || 'http://localhost:4001'

interface AuthUser {
  id: string
  email: string
  email_verified: boolean
  role: string
  metadata: string | null
  created_at: string
  updated_at: string
  banned_at: string | null
}

interface ListUsersResponse {
  users: AuthUser[]
  total: number
}

export default function AuthUsersPage() {
  const [users, setUsers] = useState<AuthUser[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  const fetchUsers = useCallback(async () => {
    try {
      const res = await fetch(`${AUTH_URL}/auth/v1/admin/users?limit=50`)
      if (res.ok) {
        const data: ListUsersResponse = await res.json()
        setUsers(data.users || [])
        setTotal(data.total)
      }
    } catch {
      // Auth service not running
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUsers()
  }, [fetchUsers])

  return (
    <div className="p-6 space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-zinc-100">Users</h2>
        <p className="text-sm text-zinc-500 mt-1">Manage authenticated users</p>
      </div>

      <Card className="border-zinc-800 bg-zinc-900">
        <CardHeader>
          <CardTitle className="text-sm text-zinc-400">
            {total} {total === 1 ? 'user' : 'users'}
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
                  <TableHead className="text-zinc-400">Status</TableHead>
                  <TableHead className="text-zinc-400">Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((user) => (
                  <TableRow key={user.id} className="border-zinc-800">
                    <TableCell className="text-zinc-300 font-mono text-sm">{user.email}</TableCell>
                    <TableCell>
                      <Badge variant={user.role === 'admin' ? 'default' : 'secondary'}>
                        {user.role}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={user.email_verified ? 'default' : 'destructive'}>
                        {user.email_verified ? 'Verified' : 'Unverified'}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {user.banned_at ? (
                        <Badge variant="destructive">Banned</Badge>
                      ) : (
                        <Badge variant="secondary">Active</Badge>
                      )}
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

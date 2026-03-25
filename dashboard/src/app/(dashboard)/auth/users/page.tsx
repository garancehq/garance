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
    fetchUsers()
  }, [])

  const fetchUsers = async () => {
    try {
      const res = await fetch(`${GATEWAY_URL}/auth/v1/admin/users`, {
        headers: { 'Authorization': `Bearer ${getToken()}` },
      })
      if (res.ok) {
        const data = await res.json()
        setUsers(data || [])
      }
    } catch {
      // Gateway not running
    } finally {
      setLoading(false)
    }
  }

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

function getToken(): string {
  return typeof window !== 'undefined' ? localStorage.getItem('garance_token') || '' : ''
}

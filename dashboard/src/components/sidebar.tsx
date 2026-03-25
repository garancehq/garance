'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { cn } from '@/lib/utils'
import {
  Database,
  Terminal,
  FileCode,
  Shield,
  HardDrive,
  Activity,
  Settings,
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

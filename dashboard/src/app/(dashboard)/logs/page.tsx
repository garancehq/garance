'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { GATEWAY_URL } from '@/lib/garance'

interface ServiceStatus {
  name: string
  url: string
  status: 'online' | 'offline' | 'checking'
}

const SERVICES: { name: string; url: string }[] = [
  { name: 'Gateway', url: `${GATEWAY_URL}/health` },
  { name: 'Engine', url: `${GATEWAY_URL}/api/v1/_tables` },
  { name: 'Auth', url: `${GATEWAY_URL}/auth/v1/signin` },
  { name: 'Storage', url: `${GATEWAY_URL}/storage/v1/buckets` },
]

export default function LogsPage() {
  const [services, setServices] = useState<ServiceStatus[]>(
    SERVICES.map((s) => ({ ...s, status: 'checking' }))
  )

  useEffect(() => {
    const checkServices = async () => {
      const results = await Promise.all(
        SERVICES.map(async (svc) => {
          try {
            const controller = new AbortController()
            const timeout = setTimeout(() => controller.abort(), 3000)
            const res = await fetch(svc.url, { signal: controller.signal })
            clearTimeout(timeout)
            // Any HTTP response (even 400/401) means the service is up
            const status: ServiceStatus['status'] = res.status > 0 ? 'online' : 'offline'
            return { ...svc, status }
          } catch {
            const status: ServiceStatus['status'] = 'offline'
            return { ...svc, status }
          }
        })
      )
      setServices(results)
    }

    checkServices()
    const interval = setInterval(checkServices, 10000)
    return () => clearInterval(interval)
  }, [])

  const onlineCount = services.filter((s) => s.status === 'online').length

  return (
    <div className="p-6 space-y-6">
      <div>
        <h2 className="text-2xl font-semibold text-zinc-100">Logs & Monitoring</h2>
        <p className="text-sm text-zinc-500 mt-1">Service health and system logs</p>
      </div>

      <Card className="border-zinc-800 bg-zinc-900">
        <CardHeader>
          <CardTitle className="text-zinc-100 text-base">
            Service Health
            <Badge variant={onlineCount === SERVICES.length ? 'default' : 'destructive'} className="ml-3">
              {onlineCount}/{SERVICES.length} online
            </Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4">
            {services.map((svc) => (
              <div
                key={svc.name}
                className="flex items-center justify-between p-3 rounded-lg border border-zinc-800 bg-zinc-950"
              >
                <div>
                  <p className="text-sm font-medium text-zinc-200">{svc.name}</p>
                  <p className="text-xs text-zinc-500 font-mono">{svc.url.replace('/health', '')}</p>
                </div>
                <Badge
                  variant={svc.status === 'online' ? 'default' : svc.status === 'checking' ? 'secondary' : 'destructive'}
                >
                  {svc.status}
                </Badge>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card className="border-zinc-800 bg-zinc-900">
        <CardHeader>
          <CardTitle className="text-zinc-100 text-base">Application Logs</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="bg-zinc-950 rounded-lg p-4 border border-zinc-800">
            <p className="text-zinc-500 text-sm mb-3">
              Real-time log streaming coming soon. For now, use the CLI:
            </p>
            <code className="text-zinc-300 text-sm font-mono bg-zinc-900 px-3 py-2 rounded block">
              garance dev logs -f
            </code>
            <p className="text-zinc-600 text-xs mt-3">
              Or directly via Docker Compose:
            </p>
            <code className="text-zinc-400 text-xs font-mono bg-zinc-900 px-3 py-1.5 rounded block mt-1">
              docker compose -f deploy/docker-compose.dev.yml logs -f
            </code>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

'use client'

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { GATEWAY_URL } from '@/lib/garance'

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
          <CardDescription>Your project&apos;s connection details</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label className="text-zinc-400">Gateway URL</Label>
            <Input
              value={GATEWAY_URL}
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

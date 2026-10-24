import { createFileRoute } from "@tanstack/react-router"
import { useQuery } from "@tanstack/react-query"
import { Badge } from "@app/ui/components/badge"
import { Button } from "@app/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@app/ui/components/card"
import { PageHeader, RequirePermission } from "@/components/page"
import { systemApi } from "@/features/admin/api"

export const Route = createFileRoute("/admin/administration/system")({
  component: SystemPage,
})

const formatUptime = (s: number) => {
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  return [d && `${d}d`, h && `${h}h`, `${m}m`].filter(Boolean).join(" ")
}

const INTEGRATIONS = {
  storage: ["Object storage", "MinIO / S3 for avatars and attachments"],
  push: ["Push notifications", "Firebase Cloud Messaging"],
  payment: ["Payment gateway", "Midtrans"],
  mailer: ["Email", "SMTP for invites, resets and verification"],
} as const

function SystemPage() {
  const status = useQuery({
    queryKey: ["system", "status"],
    queryFn: systemApi.status,
    refetchInterval: 30_000,
  })
  const data = status.data

  return (
    <RequirePermission resource="settings" action="manage">
      <PageHeader
        title="System status"
        description="Dependency health and configured integrations."
        actions={
          <Button
            variant="outline"
            disabled={status.isFetching}
            onClick={() => status.refetch()}
          >
            {status.isFetching ? "Checking…" : "Refresh"}
          </Button>
        }
      />
      {status.error && (
        <p role="alert" className="text-sm text-destructive">
          {status.error.message}
        </p>
      )}
      {data && (
        <div className="grid gap-4 lg:grid-cols-2">
          <Card>
            <CardHeader>
              <CardTitle>Runtime</CardTitle>
              <CardDescription>API process information.</CardDescription>
            </CardHeader>
            <CardContent className="grid grid-cols-3 gap-4 text-sm">
              <div>
                <p className="text-muted-foreground">Environment</p>
                <p className="font-medium">{data.environment}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Version</p>
                <p className="font-medium">{data.version ?? "—"}</p>
              </div>
              <div>
                <p className="text-muted-foreground">Uptime</p>
                <p className="font-medium">
                  {formatUptime(data.uptimeSeconds)}
                </p>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Dependencies</CardTitle>
              <CardDescription>Live checks, 2s timeout each.</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3">
              {(
                [
                  ["Database", data.checks.database],
                  ["Redis", data.checks.redis],
                ] as const
              ).map(([label, check]) => (
                <div
                  key={label}
                  className="flex items-center justify-between text-sm"
                >
                  <span>{label}</span>
                  <span className="flex items-center gap-2">
                    {check.latencyMs !== null && (
                      <span className="text-muted-foreground">
                        {check.latencyMs} ms
                      </span>
                    )}
                    <Badge
                      variant={
                        check.status === "up" ? "default" : "destructive"
                      }
                    >
                      {check.status === "up" ? "Up" : "Down"}
                    </Badge>
                  </span>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className="lg:col-span-2">
            <CardHeader>
              <CardTitle>Integrations</CardTitle>
              <CardDescription>
                Optional services; disabled ones are skipped gracefully.
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 sm:grid-cols-2">
              {(Object.keys(INTEGRATIONS) as (keyof typeof INTEGRATIONS)[]).map(
                (key) => (
                  <div
                    key={key}
                    className="flex items-start justify-between gap-3 rounded-lg border p-3 text-sm"
                  >
                    <div>
                      <p className="font-medium">{INTEGRATIONS[key][0]}</p>
                      <p className="text-muted-foreground">
                        {INTEGRATIONS[key][1]}
                      </p>
                    </div>
                    <Badge
                      variant={data.integrations[key] ? "default" : "secondary"}
                    >
                      {data.integrations[key] ? "Configured" : "Not configured"}
                    </Badge>
                  </div>
                )
              )}
            </CardContent>
          </Card>
        </div>
      )}
    </RequirePermission>
  )
}

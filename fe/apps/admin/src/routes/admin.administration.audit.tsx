import { Fragment, useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery } from "@tanstack/react-query"
import { Badge } from "@app/ui/components/badge"
import { Button } from "@app/ui/components/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@app/ui/components/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@app/ui/components/table"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import {
  Pagination,
  saveDownload,
  TableState,
  useDebounced,
} from "@/components/list-controls"
import { PageHeader, RequirePermission } from "@/components/page"
import { auditApi } from "@/features/admin/api"

export const Route = createFileRoute("/admin/administration/audit")({
  component: AuditPage,
})

const ANY = "any"
const ENTITY_TYPES = ["user", "role", "session", "settings"]

function AuditPage() {
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(20)
  const [entityType, setEntityType] = useState(ANY)
  const [action, setAction] = useState("")
  const [from, setFrom] = useState("")
  const [to, setTo] = useState("")
  const [expanded, setExpanded] = useState<string | null>(null)
  const debouncedAction = useDebounced(action)
  const filters = {
    entityType: entityType === ANY ? undefined : entityType,
    action: debouncedAction || undefined,
    from: from || undefined,
    to: to || undefined,
  }
  const params = { page, limit, ...filters }
  const audit = useQuery({
    queryKey: ["audit", params],
    queryFn: () => auditApi.list(params),
  })
  const exportAudit = useMutation({
    mutationFn: () => auditApi.export(filters),
    onSuccess: ({ blob, filename }) => saveDownload(blob, filename),
  })

  return (
    <RequirePermission resource="audit" action="view">
      <PageHeader
        title="Audit log"
        description="Every audited mutation, newest first. Secrets are redacted."
        actions={
          <Button
            variant="outline"
            disabled={exportAudit.isPending}
            onClick={() => exportAudit.mutate()}
          >
            Export CSV
          </Button>
        }
      />

      <div className="flex flex-wrap items-end gap-2">
        <Input
          placeholder="Filter action, e.g. user.update"
          aria-label="Action"
          value={action}
          onChange={(e) => {
            setAction(e.target.value)
            setPage(1)
          }}
          className="max-w-xs"
        />
        <Select
          value={entityType}
          onValueChange={(v) => {
            setEntityType(v)
            setPage(1)
          }}
        >
          <SelectTrigger className="w-44" aria-label="Entity">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>All entities</SelectItem>
            {ENTITY_TYPES.map((t) => (
              <SelectItem key={t} value={t}>
                {t}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="grid gap-1">
          <Label htmlFor="audit-from" className="text-xs">
            From
          </Label>
          <Input
            id="audit-from"
            type="date"
            value={from}
            max={to || undefined}
            onChange={(e) => {
              setFrom(e.target.value)
              setPage(1)
            }}
            className="w-40"
          />
        </div>
        <div className="grid gap-1">
          <Label htmlFor="audit-to" className="text-xs">
            To
          </Label>
          <Input
            id="audit-to"
            type="date"
            value={to}
            min={from || undefined}
            onChange={(e) => {
              setTo(e.target.value)
              setPage(1)
            }}
            className="w-40"
          />
        </div>
        {(action || from || to || entityType !== ANY) && (
          <Button
            variant="ghost"
            onClick={() => {
              setAction("")
              setFrom("")
              setTo("")
              setEntityType(ANY)
              setPage(1)
            }}
          >
            Reset
          </Button>
        )}
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Time</TableHead>
            <TableHead>Action</TableHead>
            <TableHead>Entity</TableHead>
            <TableHead>Actor</TableHead>
            <TableHead className="text-right">Changes</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableState
            colSpan={5}
            loading={audit.isPending}
            empty={audit.data?.data.length === 0}
            error={audit.error}
            emptyText="No audit entries."
          />
          {audit.data?.data.map((entry) => (
            <Fragment key={entry.id}>
              <TableRow>
                <TableCell className="whitespace-nowrap">
                  {new Date(entry.createdAt).toLocaleString()}
                </TableCell>
                <TableCell>
                  <Badge variant="outline">{entry.action}</Badge>
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {entry.entityType}/{entry.entityId}
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {entry.actorUserId ?? "system"}
                </TableCell>
                <TableCell className="text-right">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() =>
                      setExpanded(expanded === entry.id ? null : entry.id)
                    }
                  >
                    {expanded === entry.id ? "Hide" : "Show"}
                  </Button>
                </TableCell>
              </TableRow>
              {expanded === entry.id && (
                <TableRow>
                  <TableCell colSpan={5}>
                    <pre className="overflow-x-auto rounded-md bg-muted p-3 text-xs">
                      {JSON.stringify(
                        { before: entry.before, after: entry.after },
                        null,
                        2
                      )}
                    </pre>
                  </TableCell>
                </TableRow>
              )}
            </Fragment>
          ))}
        </TableBody>
      </Table>

      <Pagination
        page={page}
        totalPages={audit.data?.meta.totalPages ?? 1}
        total={audit.data?.meta.total}
        onPageChange={setPage}
        pageSize={limit}
        onPageSizeChange={(size) => {
          setLimit(size)
          setPage(1)
        }}
      />
    </RequirePermission>
  )
}

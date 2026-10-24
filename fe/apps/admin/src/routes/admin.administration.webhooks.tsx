import { useState } from "react"
import { createFileRoute } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
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
import { Pagination, TableState } from "@/components/list-controls"
import { PageHeader, RequirePermission } from "@/components/page"
import { webhooksApi } from "@/features/admin/api"

export const Route = createFileRoute("/admin/administration/webhooks")({
  component: WebhooksPage,
})

const ANY = "any"

function WebhooksPage() {
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState(ANY)
  const queryClient = useQueryClient()
  const events = useQuery({
    queryKey: ["webhooks", page, status],
    queryFn: () =>
      webhooksApi.list({
        page,
        limit: 20,
        status: status === ANY ? undefined : (status as "failed"),
      }),
  })
  const replay = useMutation({
    mutationFn: webhooksApi.replay,
    onSuccess: async () => {
      toast.success("Webhook replayed")
      await queryClient.invalidateQueries({ queryKey: ["webhooks"] })
    },
    onError: (err) => toast.error(err.message),
  })

  return (
    <RequirePermission resource="webhooks" action="manage">
      <PageHeader
        title="Webhooks"
        description="Stored provider events and failed replay."
      />
      <div className="flex gap-2">
        <Select
          value={status}
          onValueChange={(v) => {
            setStatus(v)
            setPage(1)
          }}
        >
          <SelectTrigger className="w-44">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>All statuses</SelectItem>
            <SelectItem value="received">Received</SelectItem>
            <SelectItem value="processed">Processed</SelectItem>
            <SelectItem value="failed">Failed</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Provider</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Received</TableHead>
            <TableHead>Error</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableState
            colSpan={6}
            loading={events.isLoading}
            empty={events.data?.data.length === 0}
            error={events.error}
            emptyText="No webhook events found."
          />
          {events.data?.data.map((event) => (
            <TableRow key={event.id}>
              <TableCell>{event.provider}</TableCell>
              <TableCell>{event.eventType}</TableCell>
              <TableCell>
                <Badge
                  variant={
                    event.status === "failed" ? "destructive" : "secondary"
                  }
                >
                  {event.status}
                </Badge>
              </TableCell>
              <TableCell>
                {new Date(event.receivedAt).toLocaleString()}
              </TableCell>
              <TableCell className="max-w-sm truncate">
                {event.error ?? "—"}
              </TableCell>
              <TableCell className="text-right">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={event.status !== "failed" || replay.isPending}
                  onClick={() => replay.mutate(event.id)}
                >
                  Replay
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <Pagination
        page={page}
        totalPages={events.data?.meta.totalPages ?? 1}
        onPageChange={setPage}
      />
    </RequirePermission>
  )
}

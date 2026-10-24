import { useState } from "react"
import { Link } from "@tanstack/react-router"
import { useQuery } from "@tanstack/react-query"
import { Badge } from "@app/ui/components/badge"
import { Button } from "@app/ui/components/button"
import { DetailSection } from "@/components/detail-layout"
import { ListState } from "@/components/list-controls"
import { auditApi } from "@/features/audit/api"

const PAGE_SIZE = 10

/** What this user did and what was done to them, newest first. */
export function UserActivity({ userId }: { userId: string }) {
  const [page, setPage] = useState(1)
  const params = { page, limit: PAGE_SIZE, userId }
  const activity = useQuery({
    queryKey: ["audit", params],
    queryFn: () => auditApi.list(params),
  })
  const totalPages = activity.data?.meta.totalPages ?? 1

  return (
    <DetailSection
      title="Activity"
      description="Actions by this user and changes made to their account."
    >
      <ListState
        loading={activity.isPending}
        empty={activity.data?.data.length === 0}
        error={activity.error}
        emptyText="No activity yet."
      />
      {activity.data && activity.data.data.length > 0 && (
        <ol className="grid gap-2">
          {activity.data?.data.map((entry) => (
            <li
              key={entry.id}
              className="flex flex-wrap items-center justify-between gap-2 rounded-lg border p-3 text-sm"
            >
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="outline">{entry.action}</Badge>
                <span className="text-muted-foreground">
                  {entry.actorUserId === userId
                    ? "by this user"
                    : entry.actorUserId
                      ? "by another user"
                      : "by system"}
                </span>
              </div>
              <time
                dateTime={entry.createdAt}
                className="text-xs text-muted-foreground"
              >
                {new Date(entry.createdAt).toLocaleString()}
              </time>
            </li>
          ))}
        </ol>
      )}
      <div className="mt-3 flex items-center justify-between gap-2">
        <Button asChild variant="link" className="px-0">
          <Link to="/admin/administration/audit">Open audit log</Link>
        </Button>
        {totalPages > 1 && (
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              Newer
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              Older
            </Button>
          </div>
        )}
      </div>
    </DetailSection>
  )
}

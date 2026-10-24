import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Badge } from "@app/ui/components/badge"
import { Button } from "@app/ui/components/button"
import {
  ListState,
  Pagination,
} from "@/components/list-controls"
import { PageHeader } from "@/components/page"
import { notificationsApi } from "@/features/notifications/api"

export function NotificationsPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(20)
  const [unreadOnly, setUnreadOnly] = useState(false)
  const params = { page, limit, unreadOnly }
  const list = useQuery({
    queryKey: ["notifications", params],
    queryFn: () => notificationsApi.list(params),
  })
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: ["notifications"] })
  const read = useMutation({
    mutationFn: notificationsApi.markRead,
    onSuccess: invalidate,
  })
  const readAll = useMutation({
    mutationFn: notificationsApi.readAll,
    onSuccess: invalidate,
  })
  const unread = Number(list.data?.meta.unreadCount ?? 0)

  return (
    <>
      <PageHeader
        title="Notifications"
        description={`${unread} unread`}
        actions={
          <div className="flex gap-2">
            <Button
              variant={unreadOnly ? "default" : "outline"}
              onClick={() => {
                setUnreadOnly((v) => !v)
                setPage(1)
              }}
            >
              {unreadOnly ? "Showing unread" : "Unread only"}
            </Button>
            <Button
              variant="outline"
              disabled={unread === 0 || readAll.isPending}
              onClick={() => readAll.mutate()}
            >
              Mark all read
            </Button>
          </div>
        }
      />

      <ListState
        loading={list.isPending}
        empty={list.data?.data.length === 0}
        error={list.error}
        emptyText={
          unreadOnly ? "No unread notifications." : "No notifications yet."
        }
      />
      {list.data && list.data.data.length > 0 && (
        <ul className="grid gap-2">
          {list.data?.data.map((item) => (
            <li
              key={item.id}
              className="flex items-start gap-3 rounded-lg border p-4"
            >
              <span
                aria-hidden
                className={
                  item.readAt
                    ? "mt-1.5 size-2 shrink-0 rounded-full"
                    : "mt-1.5 size-2 shrink-0 rounded-full bg-primary"
                }
              />
              <div className="min-w-0 flex-1 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{item.title}</span>
                  <Badge variant="outline">{item.type}</Badge>
                  {!item.readAt && <span className="sr-only">Unread</span>}
                </div>
                <p className="text-sm whitespace-pre-line text-muted-foreground">
                  {item.body}
                </p>
                <time
                  dateTime={item.createdAt}
                  className="text-xs text-muted-foreground"
                >
                  {new Date(item.createdAt).toLocaleString()}
                </time>
              </div>
              {!item.readAt && (
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={read.isPending}
                  onClick={() => read.mutate(item.id)}
                >
                  Mark read
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}

      <Pagination
        page={page}
        totalPages={list.data?.meta.totalPages ?? 1}
        total={list.data?.meta.total}
        onPageChange={setPage}
        pageSize={limit}
        onPageSizeChange={(size) => {
          setLimit(size)
          setPage(1)
        }}
      />
    </>
  )
}

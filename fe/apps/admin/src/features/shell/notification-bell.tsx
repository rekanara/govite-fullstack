import { BellIcon } from "lucide-react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Button } from "@app/ui/components/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@app/ui/components/dropdown-menu"
import { Link } from "@tanstack/react-router"
import { notificationsApi } from "@/features/notifications/api"

export function NotificationBell() {
  const queryClient = useQueryClient()
  const notifications = useQuery({
    queryKey: ["notifications", "bell"],
    queryFn: () => notificationsApi.list({ page: 1, limit: 10 }),
    refetchInterval: 60_000,
  })
  const unread = Number(notifications.data?.meta.unreadCount ?? 0)
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

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Notifications">
          <span className="relative">
            <BellIcon />
            {unread > 0 && (
              <span className="text-destructive-foreground absolute -top-1 -right-1 min-w-4 rounded-full bg-destructive px-1 text-[10px] leading-4">
                {unread > 9 ? "9+" : unread}
              </span>
            )}
          </span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80">
        <div className="flex items-center justify-between px-2 py-1.5">
          <DropdownMenuLabel className="p-0">Notifications</DropdownMenuLabel>
          {unread > 0 && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs"
              onClick={() => readAll.mutate()}
            >
              Mark all read
            </Button>
          )}
        </div>
        <DropdownMenuSeparator />
        {notifications.isPending ? (
          <p className="px-2 py-3 text-sm text-muted-foreground">Loading…</p>
        ) : notifications.data?.data.length ? (
          notifications.data.data.map((item) => (
            <DropdownMenuItem
              key={item.id}
              className="items-start py-2"
              onSelect={(e) => {
                e.preventDefault()
                if (!item.readAt) read.mutate(item.id)
              }}
            >
              <span
                className={
                  item.readAt
                    ? "mt-1.5 size-2 rounded-full bg-transparent"
                    : "mt-1.5 size-2 rounded-full bg-primary"
                }
              />
              <span className="grid gap-1">
                <span className="font-medium">{item.title}</span>
                <span className="line-clamp-2 text-muted-foreground">
                  {item.body}
                </span>
              </span>
            </DropdownMenuItem>
          ))
        ) : (
          <p className="px-2 py-3 text-sm text-muted-foreground">
            No notifications.
          </p>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/admin/notifications" className="justify-center">
            View all
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

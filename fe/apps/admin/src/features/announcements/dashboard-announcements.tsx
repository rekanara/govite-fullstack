import { Link } from "@tanstack/react-router"
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
import { ListState } from "@/components/list-controls"
import { publishedAnnouncementsApi } from "@/features/announcements/api"
import { useAuth } from "@/lib/auth-context"

export function DashboardAnnouncements() {
  const { can } = useAuth()
  const enabled = can("announcements", "view")
  const announcements = useQuery({
    queryKey: ["announcements", "dashboard"],
    queryFn: () => publishedAnnouncementsApi.list({ page: 1, limit: 5 }),
    enabled,
  })
  if (!enabled) return null

  return (
    <Card className="md:col-span-3">
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div>
          <CardTitle>Announcements</CardTitle>
          <CardDescription>Latest published updates.</CardDescription>
        </div>
        {can("announcements", "manage") && (
          <Button asChild variant="outline" size="sm">
            <Link to="/admin/administration/announcements">Manage</Link>
          </Button>
        )}
      </CardHeader>
      <CardContent>
        <ListState
          loading={announcements.isPending}
          empty={announcements.data?.data.length === 0}
          error={announcements.error}
          emptyText="No announcements yet."
        />
        {announcements.data && announcements.data.data.length > 0 && (
          <ol className="grid gap-3">
            {announcements.data?.data.map((item) => (
              <li key={item.id} className="rounded-lg border p-3">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="font-medium">{item.title}</p>
                  {item.isPinned && <Badge variant="outline">Pinned</Badge>}
                </div>
                <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">
                  {item.content}
                </p>
              </li>
            ))}
          </ol>
        )}
      </CardContent>
    </Card>
  )
}

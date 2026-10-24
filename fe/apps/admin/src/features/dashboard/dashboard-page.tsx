import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@app/ui/components/card"
import { PageHeader } from "@/components/page"
import { DashboardAnnouncements } from "@/features/announcements/dashboard-announcements"
import { useAuth } from "@/lib/auth-context"

export function DashboardPage() {
  const { user } = useAuth()
  return (
    <>
      <PageHeader
        title="Dashboard"
        description={`Welcome back, ${user?.fullName ?? user?.email}.`}
      />
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardDescription>Role</CardDescription>
            <CardTitle>{user?.roleName}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>Permissions</CardDescription>
            <CardTitle>{user?.permissions.length}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>Email</CardDescription>
            <CardTitle className="truncate text-base">{user?.email}</CardTitle>
          </CardHeader>
          <CardContent />
        </Card>
        <DashboardAnnouncements />
      </div>
    </>
  )
}

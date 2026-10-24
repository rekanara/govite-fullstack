import { createFileRoute } from "@tanstack/react-router"
import { UserDetailPage } from "@/features/users/user-detail-page"

export const Route = createFileRoute("/admin/administration/users_/$userId")({
  component: UserDetailPage,
})

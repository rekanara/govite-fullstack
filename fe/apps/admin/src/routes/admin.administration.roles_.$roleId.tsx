import { createFileRoute } from "@tanstack/react-router"
import { RoleDetailPage } from "@/features/roles/role-detail-page"

export const Route = createFileRoute("/admin/administration/roles_/$roleId")({
  component: RoleDetailPage,
})

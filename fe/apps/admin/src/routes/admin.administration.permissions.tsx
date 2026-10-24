import { createFileRoute } from "@tanstack/react-router"
import { PermissionsPage } from "@/features/roles/permissions-page"

export const Route = createFileRoute("/admin/administration/permissions")({
  component: PermissionsPage,
})

import { createFileRoute } from "@tanstack/react-router"
import { RolesPage } from "@/features/roles/roles-page"

export const Route = createFileRoute("/admin/administration/roles")({
  component: RolesPage,
})

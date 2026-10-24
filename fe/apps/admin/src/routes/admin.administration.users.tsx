import { createFileRoute } from "@tanstack/react-router"
import { UsersPage } from "@/features/users/users-page"

export const Route = createFileRoute("/admin/administration/users")({
  component: UsersPage,
})

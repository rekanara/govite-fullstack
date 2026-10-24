import { createFileRoute } from "@tanstack/react-router"
import { SystemPage } from "@/features/system/system-page"

export const Route = createFileRoute("/admin/administration/system")({
  component: SystemPage,
})

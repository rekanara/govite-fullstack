import { createFileRoute } from "@tanstack/react-router"
import { AnnouncementsPage } from "@/features/announcements/announcements-page"

export const Route = createFileRoute("/admin/administration/announcements")({
  component: AnnouncementsPage,
})

import { createFileRoute } from "@tanstack/react-router"
import { WebhooksPage } from "@/features/integrations/webhooks-page"

export const Route = createFileRoute("/admin/administration/webhooks")({
  component: WebhooksPage,
})

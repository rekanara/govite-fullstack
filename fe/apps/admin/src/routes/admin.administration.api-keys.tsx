import { createFileRoute } from "@tanstack/react-router"
import { ApiKeysPage } from "@/features/integrations/api-keys-page"

export const Route = createFileRoute("/admin/administration/api-keys")({
  component: ApiKeysPage,
})

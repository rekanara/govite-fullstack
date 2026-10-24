import { useQuery } from "@tanstack/react-query"
import type { PublicAppSettings } from "@app/contracts"
import { apiRequest } from "@/lib/api-client"

export function usePublicSettings() {
  return useQuery({
    queryKey: ["settings", "public"],
    queryFn: () => apiRequest<PublicAppSettings>("/settings/public"),
    staleTime: 5 * 60 * 1000,
  })
}

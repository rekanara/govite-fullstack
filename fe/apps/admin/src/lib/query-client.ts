import { QueryClient } from "@tanstack/react-query"
import { ApiError } from "./api-client"

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // 401 means no session and 403/404 won't change on retry; only retry
      // what can be transient (network, 5xx).
      retry: (failures, err) =>
        failures < 1 && !(err instanceof ApiError && err.status < 500),
    },
  },
})

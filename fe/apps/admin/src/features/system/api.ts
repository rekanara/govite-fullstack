import {
  authedRequest,
} from "@/lib/authed-request"
import type {
  SystemStatus,
} from "@app/contracts"

export const systemApi = {
  status: () => authedRequest<SystemStatus>("/system/status"),
}

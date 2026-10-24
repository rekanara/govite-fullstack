import {
  authedPage,
  authedRequest,
  qs,
} from "@/lib/authed-request"
import type {
  NotificationResponse,
} from "@app/contracts"

export const notificationsApi = {
  list: (p: { page: number; limit: number; unreadOnly?: boolean }) =>
    authedPage<NotificationResponse>(`/notifications?${qs({ ...p })}`),
  markRead: (id: string) =>
    authedRequest<NotificationResponse>(`/notifications/${id}/read`, {
      method: "POST",
    }),
  readAll: () =>
    authedRequest<void>("/notifications/read-all", { method: "POST" }),
}

import {
  authedPage,
  authedRequest,
  qs,
  fileForm,
} from "@/lib/authed-request"
import type {
  AnnouncementListQuery,
  AnnouncementResponse,
  CreateAnnouncement,
  UpdateAnnouncement,
} from "@app/contracts"

export const announcementsApi = {
  list: (p: AnnouncementListQuery) =>
    authedPage<AnnouncementResponse>(`/announcements/manage?${qs({ ...p })}`),
  get: (id: string) =>
    authedRequest<AnnouncementResponse>(`/announcements/manage/${id}`),
  create: (body: CreateAnnouncement) =>
    authedRequest<AnnouncementResponse, CreateAnnouncement>("/announcements", {
      method: "POST",
      body,
    }),
  update: (id: string, body: UpdateAnnouncement) =>
    authedRequest<AnnouncementResponse, UpdateAnnouncement>(
      `/announcements/${id}`,
      { method: "PATCH", body }
    ),
  remove: (id: string) =>
    authedRequest<void>(`/announcements/${id}`, { method: "DELETE" }),
  uploadImage: (file: File) => {
    const body = fileForm(file, "image")
    return authedRequest<{ key: string; url: string }, FormData>(
      "/announcements/image",
      { method: "POST", body }
    )
  },
}

export const publishedAnnouncementsApi = {
  list: (p: { page: number; limit: number }) =>
    authedPage<AnnouncementResponse>(`/announcements?${qs({ ...p })}`),
}

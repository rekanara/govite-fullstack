import {
  authedRequest,
  fileForm,
} from "@/lib/authed-request"
import type {
  AppSettings,
} from "@app/contracts"

export const settingsApi = {
  get: () => authedRequest<AppSettings>("/settings"),
  uploadImage: (file: File) => {
    const body = fileForm(file, "image")
    return authedRequest<{ url: string }, FormData>("/settings/image", {
      method: "POST",
      body,
    })
  },
  update: (body: AppSettings) =>
    authedRequest<AppSettings, AppSettings>("/settings", {
      method: "PUT",
      body,
    }),
}

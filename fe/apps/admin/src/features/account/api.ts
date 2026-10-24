import {
  authedRequest,
  fileForm,
} from "@/lib/authed-request"
import type {
  AttachmentResponse,
  BackupCodesResponse,
  ChangePasswordRequest,
  NotificationPreferences,
  SessionResponse,
  TwoFactorPasswordRequest,
  TwoFactorSetupResponse,
  TwoFactorStatusResponse,
  UpdateProfileRequest,
} from "@app/contracts"

export const accountApi = {
  updateProfile: (body: UpdateProfileRequest) =>
    authedRequest<{ id: string }, UpdateProfileRequest>("/account/profile", {
      method: "PATCH",
      body,
    }),
  uploadAvatar: (file: File) => {
    const body = fileForm(file)
    return authedRequest<{ avatarUrl: string | null }, FormData>(
      "/account/avatar",
      { method: "POST", body }
    )
  },
  changePassword: (body: ChangePasswordRequest) =>
    authedRequest<void, ChangePasswordRequest>("/account/password", {
      method: "POST",
      body,
    }),
  sendVerification: () =>
    authedRequest<void>("/account/verify-email", { method: "POST" }),
  twoFactor: () =>
    authedRequest<TwoFactorStatusResponse>("/account/two-factor"),
  beginTwoFactorSetup: () =>
    authedRequest<TwoFactorSetupResponse>("/account/two-factor/setup", {
      method: "POST",
    }),
  confirmTwoFactorSetup: (code: string) =>
    authedRequest<BackupCodesResponse, { code: string }>(
      "/account/two-factor/enable",
      { method: "POST", body: { code } }
    ),
  disableTwoFactor: (body: TwoFactorPasswordRequest) =>
    authedRequest<void, TwoFactorPasswordRequest>(
      "/account/two-factor/disable",
      { method: "POST", body }
    ),
  regenerateBackupCodes: (body: TwoFactorPasswordRequest) =>
    authedRequest<BackupCodesResponse, TwoFactorPasswordRequest>(
      "/account/two-factor/backup-codes",
      { method: "POST", body }
    ),
  resetTwoFactor: (body: TwoFactorPasswordRequest) =>
    authedRequest<void, TwoFactorPasswordRequest>("/account/two-factor/reset", {
      method: "POST",
      body,
    }),
  sessions: () => authedRequest<SessionResponse[]>("/account/sessions"),
  attachments: () => authedRequest<AttachmentResponse[]>("/attachments"),
  uploadAttachment: (file: File) => {
    const body = fileForm(file)
    return authedRequest<AttachmentResponse, FormData>("/attachments", {
      method: "POST",
      body,
    })
  },
  revokeSession: (id: string) =>
    authedRequest<void>(`/account/sessions/${id}`, { method: "DELETE" }),
  revokeOtherSessions: () =>
    authedRequest<void>("/account/sessions", { method: "DELETE" }),
  notificationPreferences: () =>
    authedRequest<NotificationPreferences>(
      "/account/notifications/preferences"
    ),
  updateNotificationPreferences: (body: NotificationPreferences) =>
    authedRequest<NotificationPreferences, NotificationPreferences>(
      "/account/notifications/preferences",
      { method: "PATCH", body }
    ),
  removeAttachment: (id: string) =>
    authedRequest<void>(`/attachments/${id}`, { method: "DELETE" }),
}

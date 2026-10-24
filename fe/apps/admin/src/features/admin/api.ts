import { authedDownload, authedPage, authedRequest } from "@/lib/authed-request"
import type {
  Action,
  AnnouncementListQuery,
  AnnouncementResponse,
  ApiKeyResponse,
  CreateApiKeyRequest,
  CreatedApiKeyResponse,
  CreateAnnouncement,
  UpdateAnnouncement,
  UpdateApiKeyRequest,
  AppSettings,
  BackupCodesResponse,
  TwoFactorPasswordRequest,
  TwoFactorSetupResponse,
  TwoFactorStatusResponse,
  WebhookEventQuery,
  WebhookEventResponse,
  AttachmentResponse,
  NotificationPreferences,
  NotificationResponse,
  BulkUserAction,
  BulkUserActionResult,
  ChangePasswordRequest,
  CreateRoleRequest,
  CreateUserRequest,
  PermissionDef,
  Resource,
  ResetUserPasswordRequest,
  RolePermission,
  SessionResponse,
  RoleResponse,
  SystemStatus,
  UserImportPreviewRow,
  Scope,
  UpdateProfileRequest,
  UpdateRoleRequest,
  UpdateUserRequest,
  UserResponse,
} from "@app/contracts"

const qs = (p: Record<string, string | number | boolean | undefined>) =>
  new URLSearchParams(
    Object.entries(p).flatMap(([k, v]) =>
      v === undefined || v === "" ? [] : [[k, String(v)]]
    )
  ).toString()

export interface ListParams {
  page: number
  limit: number
  search?: string
}

export type SortOrder = "asc" | "desc"

export interface UserListParams extends ListParams {
  roleId?: string
  isActive?: boolean
  deleted?: boolean
  sort?: "createdAt" | "email" | "fullName" | "lastLoginAt"
  order?: SortOrder
}

export const usersApi = {
  list: (p: UserListParams) =>
    authedPage<UserResponse>(`/users?${qs({ ...p })}`),
  get: (id: string) => authedRequest<UserResponse>(`/users/${id}`),
  create: (body: CreateUserRequest) =>
    authedRequest<UserResponse, CreateUserRequest>("/users", {
      method: "POST",
      body,
    }),
  update: (id: string, body: UpdateUserRequest) =>
    authedRequest<UserResponse, UpdateUserRequest>(`/users/${id}`, {
      method: "PATCH",
      body,
    }),
  resetPassword: (id: string, body: ResetUserPasswordRequest) =>
    authedRequest<void, ResetUserPasswordRequest>(
      `/users/${id}/reset-password`,
      { method: "POST", body }
    ),
  resendInvite: (id: string) =>
    authedRequest<void>(`/users/${id}/invite`, { method: "POST" }),
  remove: (id: string) =>
    authedRequest<UserResponse>(`/users/${id}`, { method: "DELETE" }),
  restore: (id: string) =>
    authedRequest<UserResponse>(`/users/${id}/restore`, { method: "POST" }),
  bulk: (body: BulkUserAction) =>
    authedRequest<BulkUserActionResult, BulkUserAction>("/users/bulk", {
      method: "POST",
      body,
    }),
  export: (
    p: Omit<UserListParams, "page" | "limit"> & { format: "csv" | "xlsx" }
  ) => authedDownload(`/users/export?${qs({ ...p })}`),
  importTemplate: () => authedDownload("/users/import-template"),
  previewImport: (file: File) => {
    const body = new FormData()
    body.append("file", file)
    return authedRequest<UserImportPreviewRow[], FormData>(
      "/users/import/preview",
      { method: "POST", body }
    )
  },
  import: (file: File) => {
    const body = new FormData()
    body.append("file", file)
    return authedRequest<
      { created: number; errors: { row: number; message: string }[] },
      FormData
    >("/users/import", { method: "POST", body })
  },
}

export const rolesApi = {
  list: (p: ListParams) => authedPage<RoleResponse>(`/roles?${qs({ ...p })}`),
  get: (id: string) => authedRequest<RoleResponse>(`/roles/${id}`),
  create: (body: CreateRoleRequest) =>
    authedRequest<RoleResponse, CreateRoleRequest>("/roles", {
      method: "POST",
      body,
    }),
  update: (id: string, body: UpdateRoleRequest) =>
    authedRequest<RoleResponse, UpdateRoleRequest>(`/roles/${id}`, {
      method: "PATCH",
      body,
    }),
  remove: (id: string) =>
    authedRequest<{ id: string }>(`/roles/${id}`, { method: "DELETE" }),
  setPermissions: (id: string, permissions: RolePermission[]) =>
    authedRequest<RoleResponse, { permissions: RolePermission[] }>(
      `/roles/${id}/permissions`,
      { method: "PUT", body: { permissions } }
    ),
}

export interface PermissionCatalog {
  permissions: PermissionDef[]
  resources: Resource[]
  actions: Action[]
  scopes: Scope[]
}

export const permissionsApi = {
  catalog: () => authedRequest<PermissionCatalog>("/permissions"),
}

export interface AuditEntry {
  id: string
  actorUserId: string | null
  action: string
  entityType: string
  entityId: string
  before: unknown
  after: unknown
  createdAt: string
}

export interface AuditListParams {
  page: number
  limit: number
  entityType?: string
  entityId?: string
  action?: string
  actorUserId?: string
  userId?: string
  from?: string
  to?: string
}

export const auditApi = {
  list: (p: AuditListParams) =>
    authedPage<AuditEntry>(`/audit?${qs({ ...p })}`),
  export: (p: Omit<AuditListParams, "page" | "limit">) =>
    authedDownload(`/audit/export?${qs({ ...p, page: 1, limit: 100 })}`),
}

export const accountApi = {
  updateProfile: (body: UpdateProfileRequest) =>
    authedRequest<{ id: string }, UpdateProfileRequest>("/account/profile", {
      method: "PATCH",
      body,
    }),
  uploadAvatar: (file: File) => {
    const body = new FormData()
    body.append("file", file)
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
    const body = new FormData()
    body.append("file", file)
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
    const body = new FormData()
    body.append("image", file)
    return authedRequest<{ key: string; url: string }, FormData>(
      "/announcements/image",
      { method: "POST", body }
    )
  },
}

export const settingsApi = {
  get: () => authedRequest<AppSettings>("/settings"),
  uploadImage: (file: File) => {
    const body = new FormData()
    body.append("image", file)
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

export const systemApi = {
  status: () => authedRequest<SystemStatus>("/system/status"),
}

export const webhooksApi = {
  list: (p: WebhookEventQuery) =>
    authedPage<WebhookEventResponse>(`/webhooks/events?${qs({ ...p })}`),
  replay: (id: string) =>
    authedRequest<{ duplicate: boolean }>(`/webhooks/events/${id}/replay`, {
      method: "POST",
    }),
}

export const apiKeysApi = {
  list: () => authedRequest<ApiKeyResponse[]>("/api-keys"),
  create: (body: CreateApiKeyRequest) =>
    authedRequest<CreatedApiKeyResponse, CreateApiKeyRequest>("/api-keys", {
      method: "POST",
      body,
    }),
  update: (id: string, body: UpdateApiKeyRequest) =>
    authedRequest<ApiKeyResponse, UpdateApiKeyRequest>(`/api-keys/${id}`, {
      method: "PATCH",
      body,
    }),
  remove: (id: string) =>
    authedRequest<void>(`/api-keys/${id}`, { method: "DELETE" }),
}

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

export const publishedAnnouncementsApi = {
  list: (p: { page: number; limit: number }) =>
    authedPage<AnnouncementResponse>(`/announcements?${qs({ ...p })}`),
}

import { z } from "zod"
import { ACTIONS, isKnownPermission, RESOURCES, SCOPES } from "./permissions"
import { PasswordSchema } from "./security"

export const PaginationQuerySchema = z.object({
  page: z.coerce.number().int().min(1).default(1),
  limit: z.coerce.number().int().min(1).max(100).default(20),
  search: z.string().trim().max(100).optional(),
})
export type PaginationQuery = z.infer<typeof PaginationQuerySchema>

export interface Paginated<T> {
  data: T[]
  total: number
  page: number
  limit: number
}

export const RolePermissionSchema = z
  .object({
    resource: z.enum(RESOURCES),
    action: z.enum(ACTIONS),
    scope: z.enum(SCOPES),
  })
  .refine((p) => isKnownPermission(p.resource, p.action), {
    message: "Unknown resource/action combination",
    path: ["action"],
  })
export type RolePermission = z.infer<typeof RolePermissionSchema>

export const CreateRoleRequestSchema = z.object({
  name: z.string().trim().min(1).max(50),
  description: z.string().trim().max(200).optional(),
  requireTwoFactor: z.boolean().default(false),
})
export type CreateRoleRequest = z.infer<typeof CreateRoleRequestSchema>

export const UpdateRoleRequestSchema = CreateRoleRequestSchema.partial()
export type UpdateRoleRequest = z.infer<typeof UpdateRoleRequestSchema>

export const SetRolePermissionsRequestSchema = z.object({
  permissions: z.array(RolePermissionSchema).max(200),
})
export type SetRolePermissionsRequest = z.infer<
  typeof SetRolePermissionsRequestSchema
>

export interface RoleResponse {
  id: string
  name: string
  description: string | null
  isSystem: boolean
  requireTwoFactor: boolean
  userCount: number
  permissions: RolePermission[]
}

const password = PasswordSchema

export const CreateUserRequestSchema = z.object({
  email: z.email(),
  fullName: z.string().trim().min(1).max(100),
  roleId: z.uuid(),
  password: password.optional(),
})
export type CreateUserRequest = z.infer<typeof CreateUserRequestSchema>

export const UpdateUserRequestSchema = z.object({
  fullName: z.string().trim().min(1).max(100).optional(),
  roleId: z.uuid().optional(),
  isActive: z.boolean().optional(),
})
export type UpdateUserRequest = z.infer<typeof UpdateUserRequestSchema>

export const ResetUserPasswordRequestSchema = z.object({ password })
export type ResetUserPasswordRequest = z.infer<
  typeof ResetUserPasswordRequestSchema
>

export const UserListQuerySchema = PaginationQuerySchema.extend({
  roleId: z.uuid().optional(),
  sort: z
    .enum(["createdAt", "email", "fullName", "lastLoginAt"])
    .default("createdAt"),
  order: z.enum(["asc", "desc"]).default("desc"),
  deleted: z
    .enum(["true", "false"])
    .default("false")
    .transform((v) => v === "true"),
  isActive: z
    .enum(["true", "false"])
    .transform((v) => v === "true")
    .optional(),
})
export type UserListQuery = z.infer<typeof UserListQuerySchema>

export interface UserResponse {
  id: string
  email: string
  fullName: string | null
  avatarUrl: string | null
  roleId: string
  roleName: string
  isActive: boolean
  invited: boolean
  emailVerifiedAt: string | null
  deletedAt: string | null
  lastLoginAt: string | null
  createdAt: string
}

export const UpdateProfileRequestSchema = z.object({
  fullName: z.string().trim().min(1).max(100),
})
export type UpdateProfileRequest = z.infer<typeof UpdateProfileRequestSchema>

export const ChangePasswordRequestSchema = z.object({
  currentPassword: z.string().min(1),
  newPassword: password,
})
export type ChangePasswordRequest = z.infer<typeof ChangePasswordRequestSchema>

export const TwoFactorPasswordRequestSchema = z.object({
  currentPassword: z.string().min(1),
})
export type TwoFactorPasswordRequest = z.infer<
  typeof TwoFactorPasswordRequestSchema
>

export const TwoFactorCodeRequestSchema = z.object({
  code: z.string().regex(/^\d{6}$/),
})
export type TwoFactorCodeRequest = z.infer<typeof TwoFactorCodeRequestSchema>

export interface TwoFactorSetupResponse {
  manualKey: string
  otpauthUrl: string
}

export interface TwoFactorStatusResponse {
  enabled: boolean
  required: boolean
  enabledAt: string | null
  backupCodesRemaining: number
}

export interface BackupCodesResponse {
  backupCodes: string[]
}

export interface SessionResponse {
  id: string
  userAgent: string | null
  ipAddress: string | null
  createdAt: string
  lastUsedAt: string
  current: boolean
}

export const RolePermissionResponseSchema = RolePermissionSchema

export const MeResponseSchema = z.object({
  id: z.uuid(),
  email: z.email(),
  fullName: z.string().nullable(),
  avatarUrl: z.string().nullable(),
  roleId: z.uuid(),
  roleName: z.string(),
  emailVerified: z.boolean(),
  permissions: z.array(RolePermissionResponseSchema),
})
export type MeResponse = z.infer<typeof MeResponseSchema>

export const UserExportQuerySchema = UserListQuerySchema.omit({
  page: true,
  limit: true,
}).extend({
  format: z.enum(["csv", "xlsx"]).default("csv"),
})
export type UserExportQuery = z.infer<typeof UserExportQuerySchema>

export interface UserImportResult {
  created: number
  errors: { row: number; message: string }[]
}

export const AppSettingsSchema = z.object({
  appName: z.string().trim().min(1).max(100).default("Rekanara"),
  tagline: z.string().trim().max(150).nullable().default(null),
  description: z.string().trim().max(1000).nullable().default(null),
  supportEmail: z.email().nullable().default(null),
  supportPhone: z.string().trim().max(30).nullable().default(null),
  logoUrl: z.url().nullable().default(null),
  bannerUrl: z.url().nullable().default(null),
  loginNotice: z.string().trim().max(500).nullable().default(null),
  emailFooter: z.string().trim().max(1000).nullable().default(null),
  timezone: z.string().trim().min(1).max(64).default("Asia/Jakarta"),
  currency: z.string().trim().length(3).toUpperCase().default("IDR"),
  address: z.string().trim().max(200).nullable().default(null),
  district: z.string().trim().max(100).nullable().default(null),
  city: z.string().trim().max(100).nullable().default(null),
  province: z.string().trim().max(100).nullable().default(null),
  country: z.string().trim().max(100).nullable().default(null),
  postalCode: z.string().trim().max(20).nullable().default(null),
  latitude: z.number().min(-90).max(90).nullable().default(null),
  longitude: z.number().min(-180).max(180).nullable().default(null),
  maintenanceRetentionDays: z.number().int().min(7).max(3650).default(90),
  pushNotificationsEnabled: z.boolean().default(true),
})
export type AppSettings = z.infer<typeof AppSettingsSchema>

/** Safe to show before sign-in (login page, sidebar). */
export type PublicAppSettings = Pick<
  AppSettings,
  | "appName"
  | "tagline"
  | "logoUrl"
  | "bannerUrl"
  | "loginNotice"
  | "supportEmail"
>

export const WebhookEventQuerySchema = PaginationQuerySchema.extend({
  provider: z.string().trim().max(50).optional(),
  status: z.enum(["received", "processed", "failed"]).optional(),
})
export type WebhookEventQuery = z.infer<typeof WebhookEventQuerySchema>

export interface WebhookEventResponse {
  id: string
  provider: string
  eventKey: string
  eventType: string
  status: string
  error: string | null
  payload: unknown
  receivedAt: string
  processedAt: string | null
}

export const CreateApiKeyRequestSchema = z.object({
  name: z.string().trim().min(1).max(100),
})
export type CreateApiKeyRequest = z.infer<typeof CreateApiKeyRequestSchema>

export const UpdateApiKeyRequestSchema = z.object({
  name: z.string().trim().min(1).max(100).optional(),
  isActive: z.boolean().optional(),
})
export type UpdateApiKeyRequest = z.infer<typeof UpdateApiKeyRequestSchema>

export interface ApiKeyResponse {
  id: string
  name: string
  prefix: string
  isActive: boolean
  lastUsedAt: string | null
  createdAt: string
}

export interface CreatedApiKeyResponse extends ApiKeyResponse {
  key: string
}

export interface SystemCheck {
  status: "up" | "down"
  latencyMs: number | null
}

export interface SystemStatus {
  version: string | null
  environment: string
  uptimeSeconds: number
  checks: { database: SystemCheck; redis: SystemCheck }
  integrations: {
    storage: boolean
    push: boolean
    payment: boolean
    mailer: boolean
  }
}

export const AuditQuerySchema = PaginationQuerySchema.extend({
  actorUserId: z.uuid().optional(),
  /** Everything a user did or that was done to them. */
  userId: z.uuid().optional(),
  entityType: z.string().max(50).optional(),
  entityId: z.string().max(100).optional(),
  action: z.string().max(100).optional(),
  from: z.iso.date().optional(),
  to: z.iso.date().optional(),
})
export type AuditQuery = z.infer<typeof AuditQuerySchema>

export const BulkUserActionSchema = z.object({
  ids: z.array(z.uuid()).min(1).max(100),
  action: z.enum(["activate", "deactivate", "delete", "restore"]),
})
export type BulkUserAction = z.infer<typeof BulkUserActionSchema>

export interface BulkUserActionResult {
  succeeded: string[]
  failed: { id: string; code: string; message: string }[]
}

export interface UserImportPreviewRow {
  row: number
  email: string
  fullName: string
  role: string
  errors: string[]
}

// ponytail: push only. Add an email channel here when notification emails
// exist; invite/reset/verify mails are transactional and never opt-out.
export const NotificationPreferencesSchema = z.object({
  pushEnabled: z.boolean(),
})
export type NotificationPreferences = z.infer<
  typeof NotificationPreferencesSchema
>

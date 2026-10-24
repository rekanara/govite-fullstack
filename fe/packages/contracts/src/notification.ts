import { z } from "zod"
import { PaginationQuerySchema } from "./admin"

const DeviceInfoSchema = z.record(z.string(), z.unknown())

export const RegisterFcmDeviceSchema = z.object({
  deviceId: z.string().trim().min(1).max(191),
  token: z.string().trim().min(1).max(1024),
  device: DeviceInfoSchema.default({}),
})
export type RegisterFcmDevice = z.infer<typeof RegisterFcmDeviceSchema>

export const UpdateFcmDeviceSchema = z
  .object({
    token: z.string().trim().min(1).max(1024),
    device: DeviceInfoSchema,
  })
  .partial()
  .refine((v) => v.token !== undefined || v.device !== undefined, {
    message: "Nothing to update",
  })
export type UpdateFcmDevice = z.infer<typeof UpdateFcmDeviceSchema>

export const NotificationTypeEnum = z.enum([
  "system",
  "security",
  "admin",
  "announcement",
])
export type NotificationType = z.infer<typeof NotificationTypeEnum>

export const NotificationsQuerySchema = PaginationQuerySchema.extend({
  unreadOnly: z
    .enum(["true", "false"])
    .default("false")
    .transform((v) => v === "true"),
})
export type NotificationsQuery = z.infer<typeof NotificationsQuerySchema>

export interface NotificationResponse {
  id: string
  type: NotificationType
  title: string
  body: string
  data: Record<string, string>
  readAt: string | null
  createdAt: string
}

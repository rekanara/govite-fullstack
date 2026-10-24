import { z } from "zod"
import { PaginationQuerySchema } from "./admin"

export const AnnouncementStatusEnum = z.enum(["draft", "published"])
export type AnnouncementStatus = z.infer<typeof AnnouncementStatusEnum>

export const AnnouncementListQuerySchema = PaginationQuerySchema.extend({
  status: AnnouncementStatusEnum.optional(),
})
export type AnnouncementListQuery = z.infer<typeof AnnouncementListQuerySchema>

const fields = {
  title: z.string().trim().min(1).max(200),
  content: z.string().trim().min(1).max(10000),
  imageUrl: z.url().max(2048).nullable(),
  isPinned: z.boolean(),
  status: AnnouncementStatusEnum,
}

export const CreateAnnouncementSchema = z.object({
  ...fields,
  imageUrl: fields.imageUrl.optional(),
  isPinned: fields.isPinned.default(false),
  status: fields.status.default("published"),
})
export type CreateAnnouncement = z.infer<typeof CreateAnnouncementSchema>

// No defaults here: an omitted field must stay untouched on PATCH.
export const UpdateAnnouncementSchema = z
  .object(fields)
  .partial()
  .refine((v) => Object.keys(v).length > 0, { message: "Nothing to update" })
export type UpdateAnnouncement = z.infer<typeof UpdateAnnouncementSchema>

export interface AnnouncementResponse {
  id: string
  title: string
  content: string
  imageUrl: string | null
  isPinned: boolean
  status: AnnouncementStatus
  publishedAt: string | null
  notifiedAt: string | null
  authorName: string
  createdAt: string
  updatedAt: string
}

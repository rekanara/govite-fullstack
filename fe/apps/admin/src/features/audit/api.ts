import {
  authedDownload,
  authedPage,
  qs,
} from "@/lib/authed-request"

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

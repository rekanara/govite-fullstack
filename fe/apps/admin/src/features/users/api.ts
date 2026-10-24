import {
  authedDownload,
  authedPage,
  authedRequest,
  qs,
  fileForm,
  type ListParams,
  type SortOrder,
} from "@/lib/authed-request"
import type {
  BulkUserAction,
  BulkUserActionResult,
  CreateUserRequest,
  ResetUserPasswordRequest,
  UpdateUserRequest,
  UserImportPreviewRow,
  UserResponse,
} from "@app/contracts"

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
    const body = fileForm(file)
    return authedRequest<UserImportPreviewRow[], FormData>(
      "/users/import/preview",
      { method: "POST", body }
    )
  },
  import: (file: File) => {
    const body = fileForm(file)
    return authedRequest<
      { created: number; errors: { row: number; message: string }[] },
      FormData
    >("/users/import", { method: "POST", body })
  },
}

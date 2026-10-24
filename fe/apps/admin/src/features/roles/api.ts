import {
  authedPage,
  authedRequest,
  qs,
  type ListParams,
} from "@/lib/authed-request"
import type {
  Action,
  CreateRoleRequest,
  PermissionDef,
  Resource,
  RolePermission,
  RoleResponse,
  Scope,
  UpdateRoleRequest,
} from "@app/contracts"

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

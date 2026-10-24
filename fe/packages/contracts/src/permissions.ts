import { ACTIONS, PERMISSIONS, RESOURCES, SCOPES } from "./generated"

export { ACTIONS, PERMISSIONS, RESOURCES, SCOPES }

export type Scope = (typeof SCOPES)[number]

export type Action = (typeof ACTIONS)[number]

export type Resource = (typeof RESOURCES)[number]

export interface PermissionDef {
  resource: Resource
  action: Action
}

PERMISSIONS satisfies readonly PermissionDef[]

export type Permission =
  `${(typeof PERMISSIONS)[number]["resource"]}.${(typeof PERMISSIONS)[number]["action"]}`

export function permissionKey(p: PermissionDef): Permission {
  return `${p.resource}.${p.action}` as Permission
}

export function isKnownPermission(resource: string, action: string): boolean {
  return PERMISSIONS.some((p) => p.resource === resource && p.action === action)
}

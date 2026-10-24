import { useAuth } from "./auth-context"

export function usePermission(resource: string, action: string): boolean {
  return useAuth().can(resource, action)
}

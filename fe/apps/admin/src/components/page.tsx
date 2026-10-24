import type { ReactNode } from "react"
import type { Action, Resource } from "@app/contracts"
import { useAuth } from "@/lib/auth-context"

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string
  description?: string
  actions?: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {description && (
          <p className="text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {actions}
    </div>
  )
}

/** UI-only gate; the API enforces the same permission. */
export function RequirePermission({
  resource,
  action,
  children,
}: {
  resource: Resource
  action: Action
  children: ReactNode
}) {
  if (useAuth().can(resource, action)) return children
  return (
    <div className="grid flex-1 place-items-center text-center">
      <div>
        <h2 className="text-lg font-semibold">Access denied</h2>
        <p className="text-sm text-muted-foreground">
          You need {resource}.{action} to view this page.
        </p>
      </div>
    </div>
  )
}

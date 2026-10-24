import { useState, type FormEvent } from "react"
import { Link, useNavigate, useParams } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import type { RolePermission, Scope } from "@app/contracts"
import { Button } from "@app/ui/components/button"
import { Checkbox } from "@app/ui/components/checkbox"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@app/ui/components/select"
import { ConfirmAction } from "@/components/confirm-action"
import {
  DangerZone,
  DetailField,
  DetailGrid,
  DetailHero,
  DetailSection,
} from "@/components/detail-layout"
import { RequirePermission } from "@/features/auth/require-permission"
import { permissionsApi, rolesApi } from "@/features/roles/api"

const signature = (permissions: RolePermission[]) =>
  permissions
    .map((p) => `${p.resource}.${p.action}:${p.scope}`)
    .sort()
    .join(",")

export function RoleDetailPage() {
  const { roleId } = useParams({ from: "/admin/administration/roles_/$roleId" })
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const role = useQuery({
    queryKey: ["roles", roleId],
    queryFn: () => rolesApi.get(roleId),
  })
  const catalog = useQuery({
    queryKey: ["permissions"],
    queryFn: permissionsApi.catalog,
  })
  const [edited, setDraft] = useState<RolePermission[] | null>(null)
  const draft = edited ?? role.data?.permissions ?? []
  const selectedKeys = new Set(draft.map((p) => `${p.resource}.${p.action}`))
  const dirty =
    !!role.data && signature(role.data.permissions) !== signature(draft)
  const grouped = catalog.data?.permissions.reduce(
    (acc, p) => {
      ;(acc[p.resource] ??= []).push(p)
      return acc
    },
    {} as Record<string, typeof catalog.data.permissions>
  )

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["roles"] }),
      queryClient.invalidateQueries({ queryKey: ["roles", roleId] }),
    ])
  }
  const update = useMutation({
    mutationFn: (patch: Parameters<typeof rolesApi.update>[1]) =>
      rolesApi.update(roleId, patch),
    onSuccess: async () => {
      toast.success("Role updated")
      await invalidate()
    },
  })
  const setPermissions = useMutation({
    mutationFn: (permissions: RolePermission[]) =>
      rolesApi.setPermissions(roleId, permissions),
    onSuccess: async () => {
      toast.success("Permissions saved")
      await invalidate()
      setDraft(null)
    },
  })
  const remove = useMutation({
    mutationFn: () => rolesApi.remove(roleId),
    onSuccess: async () => {
      toast.success("Role deleted")
      await queryClient.invalidateQueries({ queryKey: ["roles"] })
      await navigate({ to: "/admin/administration/roles" })
    },
  })

  function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    update.mutate({
      name: String(form.get("name")),
      description: String(form.get("description") || ""),
      requireTwoFactor: form.get("requireTwoFactor") === "on",
    })
  }

  function togglePermission(permission: RolePermission, checked: boolean) {
    const key = `${permission.resource}.${permission.action}`
    const next = draft.filter((p) => `${p.resource}.${p.action}` !== key)
    if (checked) next.push(permission)
    setDraft(next)
  }

  return (
    <RequirePermission resource="roles" action="manage">
      {role.isPending && (
        <p className="text-sm text-muted-foreground">Loading…</p>
      )}
      {role.data && (
        <>
          <DetailHero
            eyebrow="Role detail"
            title={role.data.name}
            description={
              role.data.description || "Configure access for this role."
            }
            badges={
              <>
                <span className="rounded-full border px-2.5 py-0.5 text-xs font-medium">
                  {role.data.isSystem ? "System" : "Custom"}
                </span>
                <span className="rounded-full border px-2.5 py-0.5 text-xs font-medium">
                  {role.data.permissions.length} permissions
                </span>
                <span className="rounded-full border px-2.5 py-0.5 text-xs font-medium">
                  {role.data.userCount} users
                </span>
              </>
            }
            actions={
              <Button asChild variant="outline">
                <Link to="/admin/administration/roles">Back to roles</Link>
              </Button>
            }
          />

          <DetailGrid>
            <div className="grid gap-4">
              <DetailSection
                title="Role profile"
                description="System roles are protected from metadata edits."
              >
                <form onSubmit={onSubmit} className="grid gap-4">
                  <div className="grid gap-2">
                    <Label htmlFor="name">Name</Label>
                    <Input
                      id="name"
                      name="name"
                      defaultValue={role.data.name}
                      disabled={role.data.isSystem}
                      required
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="description">Description</Label>
                    <Input
                      id="description"
                      name="description"
                      defaultValue={role.data.description ?? ""}
                      disabled={role.data.isSystem}
                    />
                  </div>
                  <label className="flex items-center gap-2 text-sm">
                    <Checkbox
                      name="requireTwoFactor"
                      defaultChecked={role.data.requireTwoFactor}
                      disabled={role.data.isSystem}
                    />
                    Require 2FA
                  </label>
                  <Button
                    type="submit"
                    disabled={role.data.isSystem || update.isPending}
                  >
                    Save profile
                  </Button>
                </form>
              </DetailSection>

              <DetailSection title="Overview">
                <div className="grid gap-3 sm:grid-cols-2">
                  <DetailField
                    label="Type"
                    value={role.data.isSystem ? "System" : "Custom"}
                  />
                  <DetailField label="Users" value={role.data.userCount} />
                  <DetailField
                    label="2FA"
                    value={role.data.requireTwoFactor ? "Required" : "Optional"}
                  />
                  <DetailField
                    label="Permissions"
                    value={role.data.permissions.length}
                  />
                  <DetailField
                    label="Pending edits"
                    value={dirty ? "Unsaved changes" : "None"}
                  />
                </div>
              </DetailSection>

              <DangerZone>
                <ConfirmAction
                  title={`Delete role "${role.data.name}"?`}
                  description="This cannot be undone."
                  confirmLabel="Delete"
                  onConfirm={() => remove.mutate()}
                  trigger={
                    <Button
                      variant="destructive"
                      disabled={role.data.isSystem || role.data.userCount > 0}
                    >
                      Delete role
                    </Button>
                  }
                />
              </DangerZone>
            </div>

            <DetailSection
              title="Permissions"
              description="Adjust permissions, then save all changes together."
              actions={
                <Button
                  size="sm"
                  onClick={() => setPermissions.mutate(draft)}
                  disabled={
                    role.data.isSystem || !dirty || setPermissions.isPending
                  }
                >
                  Save permissions
                </Button>
              }
            >
              <div className="grid gap-4">
                {Object.entries(grouped ?? {}).map(
                  ([resource, permissions]) => (
                    <div key={resource} className="rounded-xl border p-3">
                      <div className="mb-3 flex items-center justify-between">
                        <h3 className="font-mono text-sm font-semibold">
                          {resource}
                        </h3>
                        <span className="text-xs text-muted-foreground">
                          {permissions.length} actions
                        </span>
                      </div>
                      <div className="grid gap-3">
                        {permissions.map((p) => {
                          const key = `${p.resource}.${p.action}`
                          const existing = draft.find(
                            (x) => `${x.resource}.${x.action}` === key
                          )
                          return (
                            <div
                              key={key}
                              className="flex items-center justify-between gap-3 rounded-lg bg-muted/30 p-2"
                            >
                              <Label className="flex items-center gap-2 font-mono text-sm">
                                <Checkbox
                                  checked={selectedKeys.has(key)}
                                  disabled={role.data.isSystem}
                                  onCheckedChange={(checked) =>
                                    togglePermission(
                                      { ...p, scope: existing?.scope ?? "all" },
                                      checked === true
                                    )
                                  }
                                />
                                {p.action}
                              </Label>
                              <Select
                                value={existing?.scope ?? "all"}
                                disabled={role.data.isSystem || !existing}
                                onValueChange={(scope) =>
                                  setDraft(
                                    draft.map((permission) =>
                                      `${permission.resource}.${permission.action}` ===
                                      key
                                        ? {
                                            ...permission,
                                            scope: scope as Scope,
                                          }
                                        : permission
                                    )
                                  )
                                }
                              >
                                <SelectTrigger className="w-28">
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  {catalog.data?.scopes.map((scope) => (
                                    <SelectItem key={scope} value={scope}>
                                      {scope}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  )
                )}
              </div>
            </DetailSection>
          </DetailGrid>
        </>
      )}
    </RequirePermission>
  )
}

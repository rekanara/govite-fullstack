import { type FormEvent } from "react"
import { SECURITY_POLICY } from "@app/contracts"
import { Link, useParams } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Button } from "@app/ui/components/button"
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
  StatusBadge,
} from "@/components/detail-layout"
import { RequirePermission } from "@/features/auth/require-permission"
import { rolesApi } from "@/features/roles/api"
import { usersApi } from "@/features/users/api"
import { UserActivity } from "@/features/audit/user-activity"
import { useAuth } from "@/lib/auth-context"

export function UserDetailPage() {
  const { userId } = useParams({ from: "/admin/administration/users_/$userId" })
  const { can } = useAuth()
  const queryClient = useQueryClient()
  const user = useQuery({
    queryKey: ["users", userId],
    queryFn: () => usersApi.get(userId),
  })
  const roles = useQuery({
    queryKey: ["roles", "options"],
    queryFn: () => rolesApi.list({ page: 1, limit: 100 }),
  })
  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["users"] }),
      queryClient.invalidateQueries({ queryKey: ["users", userId] }),
    ])
  }
  const update = useMutation({
    mutationFn: (patch: Parameters<typeof usersApi.update>[1]) =>
      usersApi.update(userId, patch),
    onSuccess: async () => {
      toast.success("User updated")
      await invalidate()
    },
  })
  const resetPassword = useMutation({
    mutationFn: (password: string) =>
      usersApi.resetPassword(userId, { password }),
    onSuccess: async () => {
      toast.success("Password reset")
      await invalidate()
    },
  })
  const resendInvite = useMutation({
    mutationFn: () => usersApi.resendInvite(userId),
    onSuccess: () => toast.success("Invite sent"),
  })
  const remove = useMutation({
    mutationFn: () => usersApi.remove(userId),
    onSuccess: async () => {
      toast.success("User deleted")
      await invalidate()
    },
  })
  const restore = useMutation({
    mutationFn: () => usersApi.restore(userId),
    onSuccess: async () => {
      toast.success("User restored")
      await invalidate()
    },
  })

  function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    update.mutate({
      fullName: String(form.get("fullName")),
      roleId: String(form.get("roleId")),
    })
  }

  function onResetPassword(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    resetPassword.mutate(String(form.get("password")))
  }

  return (
    <RequirePermission resource="users" action="manage">
      {user.isPending && (
        <p className="text-sm text-muted-foreground">Loading…</p>
      )}
      {user.data && (
        <>
          <DetailHero
            eyebrow="User detail"
            title={user.data.fullName ?? user.data.email}
            description={user.data.email}
            badges={
              <>
                <StatusBadge active={user.data.isActive} />
                <span className="rounded-full border px-2.5 py-0.5 text-xs font-medium">
                  {user.data.roleName}
                </span>
              </>
            }
            actions={
              <Button asChild variant="outline">
                <Link to="/admin/administration/users">Back to users</Link>
              </Button>
            }
          />

          <DetailGrid>
            <div className="grid gap-4">
              <DetailSection
                title="Profile"
                description="Core identity and role assignment."
              >
                <form onSubmit={onSubmit} className="grid gap-4">
                  <div className="grid gap-2">
                    <Label htmlFor="email">Email</Label>
                    <Input id="email" value={user.data.email} disabled />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="fullName">Full name</Label>
                    <Input
                      id="fullName"
                      name="fullName"
                      defaultValue={user.data.fullName ?? ""}
                      required
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="roleId">Role</Label>
                    <Select
                      name="roleId"
                      defaultValue={user.data.roleId}
                      required
                    >
                      <SelectTrigger id="roleId">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {roles.data?.data.map((role) => (
                          <SelectItem key={role.id} value={role.id}>
                            {role.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <Button type="submit" disabled={update.isPending}>
                    Save profile
                  </Button>
                </form>
              </DetailSection>

              <DangerZone>
                {user.data.deletedAt ? (
                  <Button onClick={() => restore.mutate()}>Restore user</Button>
                ) : user.data.isActive ? (
                  <ConfirmAction
                    title={`Deactivate ${user.data.email}?`}
                    description="User will be signed out and unable to log in."
                    confirmLabel="Deactivate"
                    onConfirm={() => update.mutate({ isActive: false })}
                    trigger={
                      <Button variant="destructive">Deactivate user</Button>
                    }
                  />
                ) : (
                  <Button onClick={() => update.mutate({ isActive: true })}>
                    Activate user
                  </Button>
                )}
                {!user.data.deletedAt && (
                  <ConfirmAction
                    title={`Delete ${user.data.email}?`}
                    description="User will be hidden from active lists and signed out."
                    confirmLabel="Delete"
                    onConfirm={() => remove.mutate()}
                    trigger={<Button variant="destructive">Delete user</Button>}
                  />
                )}
              </DangerZone>
            </div>

            <div className="grid gap-4">
              <DetailSection title="Account overview">
                <div className="grid gap-3 sm:grid-cols-2">
                  <DetailField
                    label="Status"
                    value={<StatusBadge active={user.data.isActive} />}
                  />
                  <DetailField label="Role" value={user.data.roleName} />
                  <DetailField
                    label="Email"
                    value={
                      user.data.emailVerifiedAt ? "Verified" : "Unverified"
                    }
                  />
                  {user.data.invited && (
                    <DetailField
                      label="Invite"
                      value={
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={resendInvite.isPending}
                          onClick={() => resendInvite.mutate()}
                        >
                          Resend invite
                        </Button>
                      }
                    />
                  )}
                  <DetailField
                    label="Last login"
                    value={
                      user.data.lastLoginAt
                        ? new Date(user.data.lastLoginAt).toLocaleString()
                        : "—"
                    }
                  />
                  <DetailField
                    label="Created"
                    value={new Date(user.data.createdAt).toLocaleString()}
                  />
                </div>
              </DetailSection>

              <DetailSection
                title="Security"
                description="Reset password and revoke active sessions."
              >
                <form onSubmit={onResetPassword} className="grid gap-3">
                  <div className="grid gap-2">
                    <Label htmlFor="password">New password</Label>
                    <Input
                      id="password"
                      name="password"
                      type="password"
                      minLength={SECURITY_POLICY.passwordMinLength}
                      maxLength={SECURITY_POLICY.passwordMaxLength}
                      required
                    />
                  </div>
                  <Button
                    type="submit"
                    variant="outline"
                    disabled={resetPassword.isPending}
                  >
                    Reset password
                  </Button>
                </form>
              </DetailSection>

              {can("audit", "view") && <UserActivity userId={userId} />}
            </div>
          </DetailGrid>
        </>
      )}
    </RequirePermission>
  )
}

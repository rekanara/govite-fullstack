import { useState, type FormEvent } from "react"
import { SECURITY_POLICY } from "@app/contracts"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import {
  Avatar,
  AvatarFallback,
  AvatarImage,
} from "@app/ui/components/avatar"
import { Button } from "@app/ui/components/button"
import { Checkbox } from "@app/ui/components/checkbox"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@app/ui/components/card"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import { ConfirmAction } from "@/components/confirm-action"
import { PageHeader } from "@/components/page"
import { accountApi } from "@/features/account/api"
import { TwoFactorCard } from "@/features/account/two-factor-card"
import { useAuth } from "@/lib/auth-context"

export function AccountPage() {
  const { user, refreshUser } = useAuth()
  const queryClient = useQueryClient()
  const [profileName, setProfileName] = useState(user?.fullName ?? "")
  const profile = useMutation({
    mutationFn: accountApi.updateProfile,
    onSuccess: async () => {
      toast.success("Profile updated")
      await refreshUser()
    },
  })
  const avatar = useMutation({
    mutationFn: accountApi.uploadAvatar,
    onSuccess: async () => {
      toast.success("Avatar updated")
      await refreshUser()
    },
  })
  const password = useMutation({
    mutationFn: accountApi.changePassword,
    onSuccess: () => toast.success("Password changed"),
  })
  const sessions = useQuery({
    queryKey: ["account", "sessions"],
    queryFn: accountApi.sessions,
  })
  const attachments = useQuery({
    queryKey: ["account", "attachments"],
    queryFn: accountApi.attachments,
  })
  const uploadAttachment = useMutation({
    mutationFn: accountApi.uploadAttachment,
    onSuccess: async () => {
      toast.success("Attachment uploaded")
      await queryClient.invalidateQueries({
        queryKey: ["account", "attachments"],
      })
    },
  })
  const revokeSession = useMutation({
    mutationFn: accountApi.revokeSession,
    onSuccess: async () => {
      toast.success("Session revoked")
      await queryClient.invalidateQueries({ queryKey: ["account", "sessions"] })
    },
  })
  const preferences = useQuery({
    queryKey: ["account", "notification-preferences"],
    queryFn: accountApi.notificationPreferences,
  })
  const updatePreferences = useMutation({
    mutationFn: accountApi.updateNotificationPreferences,
    onSuccess: async () => {
      toast.success("Notification preferences saved")
      await queryClient.invalidateQueries({
        queryKey: ["account", "notification-preferences"],
      })
    },
  })
  const removeAttachment = useMutation({
    mutationFn: accountApi.removeAttachment,
    onSuccess: async () => {
      toast.success("Attachment deleted")
      await queryClient.invalidateQueries({
        queryKey: ["account", "attachments"],
      })
    },
  })
  const revokeOthers = useMutation({
    mutationFn: accountApi.revokeOtherSessions,
    onSuccess: async () => {
      toast.success("Other sessions signed out")
      await queryClient.invalidateQueries({ queryKey: ["account", "sessions"] })
    },
  })
  const otherSessions = (sessions.data ?? []).filter((s) => !s.current).length
  const verifyEmail = useMutation({
    mutationFn: accountApi.sendVerification,
    onSuccess: () => toast.success("Verification email sent"),
  })

  function updateProfile(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    profile.mutate({ fullName: profileName })
  }

  function changePassword(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = e.currentTarget
    const data = new FormData(form)
    password.mutate(
      {
        currentPassword: String(data.get("currentPassword")),
        newPassword: String(data.get("newPassword")),
      },
      { onSuccess: () => form.reset() }
    )
  }

  return (
    <>
      <PageHeader
        title="Account"
        description="Manage your profile, password and sessions."
      />
      <div className="grid items-start gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Profile</CardTitle>
            <CardDescription className="flex flex-wrap items-center gap-2">
              <span>{user?.email}</span>
              {user?.emailVerified ? (
                <span className="rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs text-emerald-700">
                  Verified
                </span>
              ) : (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => verifyEmail.mutate()}
                  disabled={verifyEmail.isPending}
                >
                  Verify email
                </Button>
              )}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={updateProfile} className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="avatar">Avatar</Label>
                <div className="flex items-center gap-3">
                  <Avatar size="lg">
                    {user?.avatarUrl && (
                      <AvatarImage src={user.avatarUrl} alt="" />
                    )}
                    <AvatarFallback>
                      {(user?.fullName ?? user?.email ?? "?")
                        .slice(0, 2)
                        .toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                  <Input
                    id="avatar"
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    onChange={(e) => {
                      const file = e.currentTarget.files?.[0]
                      if (file) avatar.mutate(file)
                    }}
                  />
                </div>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="fullName">Full name</Label>
                <Input
                  id="fullName"
                  value={profileName}
                  onChange={(e) => setProfileName(e.target.value)}
                  required
                />
              </div>
              <Button className="w-fit" disabled={profile.isPending}>
                Save profile
              </Button>
            </form>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Password</CardTitle>
            <CardDescription>
              Changing password signs out other sessions.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={changePassword} className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="currentPassword">Current password</Label>
                <Input
                  id="currentPassword"
                  name="currentPassword"
                  type="password"
                  required
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="newPassword">New password</Label>
                <Input
                  id="newPassword"
                  name="newPassword"
                  type="password"
                  minLength={SECURITY_POLICY.passwordMinLength}
                  maxLength={SECURITY_POLICY.passwordMaxLength}
                  required
                />
              </div>
              <Button className="w-fit" disabled={password.isPending}>
                Change password
              </Button>
            </form>
          </CardContent>
        </Card>
        <TwoFactorCard />
        <Card>
          <CardHeader>
            <CardTitle>Notifications</CardTitle>
            <CardDescription>
              In-app notifications are always kept. This controls push to your
              devices.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <label className="flex items-center gap-3 text-sm">
              <Checkbox
                checked={preferences.data?.pushEnabled ?? true}
                disabled={preferences.isPending || updatePreferences.isPending}
                onCheckedChange={(checked) =>
                  updatePreferences.mutate({ pushEnabled: checked === true })
                }
              />
              Push notifications on my devices
            </label>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Attachments</CardTitle>
            <CardDescription>
              Private files attached to your account.
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <Input
              type="file"
              onChange={(e) => {
                const file = e.currentTarget.files?.[0]
                if (file) uploadAttachment.mutate(file)
              }}
            />
            <div className="grid gap-2">
              {(attachments.data ?? []).map((file) => (
                <div
                  key={file.id}
                  className="flex items-center justify-between gap-3 rounded-lg border p-3 text-sm"
                >
                  <a href={file.url} className="min-w-0 hover:underline">
                    <span className="font-medium">{file.filename}</span>
                    <span className="ml-2 text-muted-foreground">
                      {Math.ceil(file.size / 1024)} KB
                    </span>
                  </a>
                  <ConfirmAction
                    title={`Delete ${file.filename}?`}
                    description="The file is removed from storage permanently."
                    confirmLabel="Delete"
                    onConfirm={() => removeAttachment.mutate(file.id)}
                    trigger={
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={removeAttachment.isPending}
                      >
                        Delete
                      </Button>
                    }
                  />
                </div>
              ))}
              {attachments.data?.length === 0 && (
                <p className="text-sm text-muted-foreground">
                  No attachments yet.
                </p>
              )}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="space-y-1.5">
                <CardTitle>Sessions</CardTitle>
                <CardDescription>
                  Active sign-ins on this account.
                </CardDescription>
              </div>
              {otherSessions > 0 && (
                <ConfirmAction
                  title="Sign out all other sessions?"
                  description="Every other device will need to sign in again. This one stays signed in."
                  confirmLabel="Sign out others"
                  onConfirm={() => revokeOthers.mutate()}
                  trigger={
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={revokeOthers.isPending}
                    >
                      Sign out {otherSessions} other
                      {otherSessions === 1 ? "" : "s"}
                    </Button>
                  }
                />
              )}
            </div>
          </CardHeader>
          <CardContent className="grid gap-3">
            {(sessions.data ?? []).map((session) => (
              <div
                key={session.id}
                className="flex items-center justify-between gap-3 rounded-lg border p-3 text-sm"
              >
                <div className="min-w-0">
                  <p className="truncate font-medium">
                    {session.userAgent ?? "Unknown device"}
                  </p>
                  <p className="text-muted-foreground">
                    {session.current ? "Current session" : session.ipAddress}
                  </p>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={session.current || revokeSession.isPending}
                  onClick={() => revokeSession.mutate(session.id)}
                >
                  Revoke
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    </>
  )
}

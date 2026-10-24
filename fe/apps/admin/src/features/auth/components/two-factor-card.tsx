import { useState, type FormEvent } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { QRCodeSVG } from "qrcode.react"
import { Button } from "@app/ui/components/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@app/ui/components/card"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import { accountApi } from "@/features/admin/api"
import { ApiError } from "@/lib/api-client"
import { BackupCodes } from "./backup-codes"

type Action = "enable" | "regenerate" | "reset" | "disable"

export function TwoFactorCard() {
  const queryClient = useQueryClient()
  const [action, setAction] = useState<Action | null>(null)
  const [setup, setSetup] = useState<{
    manualKey: string
    otpauthUrl: string
  } | null>(null)
  const [codes, setCodes] = useState<string[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const status = useQuery({
    queryKey: ["account", "two-factor"],
    queryFn: accountApi.twoFactor,
  })
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["account", "two-factor"] })

  const startSetup = useMutation({
    mutationFn: accountApi.beginTwoFactorSetup,
    onSuccess: (res) => {
      setSetup(res)
      setAction("enable")
    },
  })
  const confirmSetup = useMutation({
    mutationFn: accountApi.confirmTwoFactorSetup,
    onSuccess: async (res) => {
      setCodes(res.backupCodes)
      setSetup(null)
      setAction(null)
      await refresh()
    },
    onError: () => setError("Invalid code."),
  })
  const regenerate = useMutation({
    mutationFn: accountApi.regenerateBackupCodes,
    onSuccess: async (res) => {
      setCodes(res.backupCodes)
      setAction(null)
      await refresh()
    },
    onError: () => setError("Incorrect password."),
  })
  const reset = useMutation({
    mutationFn: accountApi.resetTwoFactor,
    onSuccess: async () => {
      toast.success("Two-factor reset; set it up again at next sign-in")
      setAction(null)
      setCodes(null)
      await refresh()
    },
    onError: () => setError("Incorrect password."),
  })
  const disable = useMutation({
    mutationFn: accountApi.disableTwoFactor,
    onSuccess: async () => {
      toast.success("Two-factor disabled")
      setAction(null)
      await refresh()
    },
    onError: (err) =>
      setError(
        err instanceof ApiError && err.status === 403
          ? "Your role requires two-factor authentication."
          : "Incorrect password."
      ),
  })

  function submitPassword(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    const currentPassword = String(
      new FormData(e.currentTarget).get("currentPassword")
    )
    ;(action === "reset" ? reset : disable).mutate({ currentPassword })
  }

  function submitCode(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    confirmSetup.mutate(String(new FormData(e.currentTarget).get("code")))
  }

  function cancel() {
    setAction(null)
    setSetup(null)
    setError(null)
  }

  if (!status.data) return null
  const { enabled, required, enabledAt, backupCodesRemaining } = status.data

  return (
    <Card>
      <CardHeader>
        <CardTitle>Two-factor authentication</CardTitle>
        <CardDescription>
          {enabled
            ? `On since ${new Date(enabledAt!).toLocaleDateString()}. ${backupCodesRemaining} backup codes left.`
            : required
              ? "Required for your role. You will set it up at next sign-in."
              : "Not enabled."}
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        {codes && <BackupCodes codes={codes} />}

        {!enabled && !action && (
          <Button
            size="sm"
            className="w-fit"
            onClick={() => startSetup.mutate()}
            disabled={startSetup.isPending}
          >
            Enable two-factor
          </Button>
        )}

        {action === "enable" && setup && (
          <form onSubmit={submitCode} className="grid gap-3">
            <p className="text-sm text-muted-foreground">
              Scan with your authenticator app, then enter the 6-digit code.
            </p>
            <div className="flex justify-center rounded-md bg-white p-3">
              <QRCodeSVG value={setup.otpauthUrl} size={176} />
            </div>
            <p className="font-mono text-xs break-all">{setup.manualKey}</p>
            <div className="grid gap-2">
              <Label htmlFor="setup-code">Authentication code</Label>
              <Input
                id="setup-code"
                name="code"
                inputMode="numeric"
                pattern="[0-9]{6}"
                maxLength={6}
                required
              />
            </div>
            {error && (
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
            )}
            <div className="flex gap-2">
              <Button type="submit" size="sm" disabled={confirmSetup.isPending}>
                Confirm
              </Button>
              <Button type="button" size="sm" variant="ghost" onClick={cancel}>
                Cancel
              </Button>
            </div>
          </form>
        )}

        {enabled && !action && (
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setAction("regenerate")}
            >
              New backup codes
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setAction("reset")}
            >
              Reset two-factor
            </Button>
            {!required && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setAction("disable")}
              >
                Disable
              </Button>
            )}
          </div>
        )}

        {action === "regenerate" && (
          <form
            onSubmit={(e) => {
              e.preventDefault()
              setError(null)
              regenerate.mutate({
                currentPassword: String(
                  new FormData(e.currentTarget).get("currentPassword")
                ),
              })
            }}
            className="grid gap-3"
          >
            <p className="text-sm text-muted-foreground">
              Old backup codes stop working. Enter your password to confirm.
            </p>
            <PasswordField error={error} />
            <Actions pending={regenerate.isPending} onCancel={cancel} />
          </form>
        )}

        {(action === "reset" || action === "disable") && (
          <form onSubmit={submitPassword} className="grid gap-3">
            <p className="text-sm text-muted-foreground">
              {action === "reset"
                ? "Removes your authenticator and backup codes, and signs out other sessions. Enter your password to confirm."
                : "Turns two-factor off and signs out other sessions. Enter your password to confirm."}
            </p>
            <PasswordField error={error} />
            <Actions
              pending={(action === "reset" ? reset : disable).isPending}
              destructive
              onCancel={cancel}
            />
          </form>
        )}
      </CardContent>
    </Card>
  )
}

function PasswordField({ error }: { error: string | null }) {
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="tf-password">Current password</Label>
        <Input
          id="tf-password"
          name="currentPassword"
          type="password"
          autoComplete="current-password"
          required
        />
      </div>
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </>
  )
}

function Actions({
  pending,
  destructive,
  onCancel,
}: {
  pending: boolean
  destructive?: boolean
  onCancel: () => void
}) {
  return (
    <div className="flex gap-2">
      <Button
        type="submit"
        size="sm"
        variant={destructive ? "destructive" : "default"}
        disabled={pending}
      >
        Confirm
      </Button>
      <Button type="button" size="sm" variant="ghost" onClick={onCancel}>
        Cancel
      </Button>
    </div>
  )
}

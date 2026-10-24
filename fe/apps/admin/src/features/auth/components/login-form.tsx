import { useRef, useState, type FormEvent } from "react"
import { SECURITY_POLICY } from "@app/contracts"
import { Link, useNavigate, useRouter } from "@tanstack/react-router"
import { QRCodeSVG } from "qrcode.react"
import { Button } from "@app/ui/components/button"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import { useAuth } from "@/lib/auth-context"
import { ApiError } from "@/lib/api-client"
import { BackupCodes } from "./backup-codes"

export function LoginForm({ redirectTo }: { redirectTo?: string }) {
  const { login, verifyTwoFactor, verifyTwoFactorBackup } = useAuth()
  const navigate = useNavigate()
  const router = useRouter()
  const passwordRef = useRef<HTMLInputElement>(null)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)
  const [useBackup, setUseBackup] = useState(false)
  const [backupCodes, setBackupCodes] = useState<string[] | null>(null)
  const [challenge, setChallenge] = useState<{
    token: string
    setupRequired: boolean
    manualKey?: string
    otpauthUrl?: string
  } | null>(null)

  async function finish() {
    if (redirectTo) router.history.push(redirectTo)
    else await navigate({ to: "/admin/dashboard" })
  }

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    setError(null)
    setPending(true)
    try {
      const result = await login(
        String(form.get("email")),
        String(form.get("password"))
      )
      if (result.requiresTwoFactor) {
        setChallenge({
          token: result.challengeToken,
          setupRequired: result.setupRequired,
          manualKey: result.manualKey,
          otpauthUrl: result.otpauthUrl,
        })
        return
      }
      await finish()
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 429
          ? "Too many attempts. Try again in a minute."
          : "Incorrect email or password."
      )
      if (passwordRef.current) passwordRef.current.value = ""
    } finally {
      setPending(false)
    }
  }

  async function onTwoFactor(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!challenge) return
    const form = new FormData(e.currentTarget)
    setError(null)
    setPending(true)
    try {
      const result = useBackup
        ? await verifyTwoFactorBackup(
            challenge.token,
            String(form.get("backupCode"))
          )
        : await verifyTwoFactor(challenge.token, String(form.get("code")))
      if (result.requiresTwoFactor) return
      if (result.backupCodes?.length) setBackupCodes(result.backupCodes)
      else await finish()
    } catch {
      setError(useBackup ? "Invalid backup code." : "Invalid two-factor code.")
    } finally {
      setPending(false)
    }
  }

  if (backupCodes) {
    return (
      <div className="space-y-4">
        <p className="font-medium">Two-factor authentication is on</p>
        <BackupCodes codes={backupCodes} />
        <Button size="lg" className="w-full" onClick={() => void finish()}>
          I saved my codes, continue
        </Button>
      </div>
    )
  }

  if (challenge) {
    return (
      <form onSubmit={onTwoFactor} className="space-y-4">
        {challenge.setupRequired && challenge.manualKey && (
          <div className="space-y-3 rounded-lg border p-3 text-sm">
            <p className="font-medium">Set up two-factor authentication</p>
            <p className="text-muted-foreground">
              Scan this QR code with your authenticator app (Google
              Authenticator, Authy, 1Password…), or enter the key manually.
            </p>
            {challenge.otpauthUrl && (
              <div className="flex justify-center rounded-md bg-white p-3">
                <QRCodeSVG value={challenge.otpauthUrl} size={176} />
              </div>
            )}
            <p className="font-mono text-xs break-all">{challenge.manualKey}</p>
          </div>
        )}
        {useBackup ? (
          <div className="space-y-1.5">
            <Label htmlFor="backupCode">Backup code</Label>
            <Input
              id="backupCode"
              name="backupCode"
              autoComplete="off"
              placeholder="12345-67890"
              maxLength={32}
              required
            />
          </div>
        ) : (
          <div className="space-y-1.5">
            <Label htmlFor="code">Authentication code</Label>
            <Input
              id="code"
              name="code"
              inputMode="numeric"
              pattern="[0-9]{6}"
              autoComplete="one-time-code"
              maxLength={6}
              required
            />
          </div>
        )}
        {!challenge.setupRequired && (
          <button
            type="button"
            className="text-xs text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
            onClick={() => {
              setUseBackup((v) => !v)
              setError(null)
            }}
          >
            {useBackup
              ? "Use authenticator code instead"
              : "Lost your device? Use a backup code"}
          </button>
        )}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <Button type="submit" size="lg" className="w-full" disabled={pending}>
          {pending ? "Verifying…" : "Verify"}
        </Button>
      </form>
    )
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <div className="space-y-1.5">
        <Label htmlFor="email">Email</Label>
        <Input
          id="email"
          name="email"
          type="email"
          autoComplete="email"
          placeholder="name@company.com"
          required
        />
      </div>
      <div className="space-y-1.5">
        <div className="flex items-center justify-between">
          <Label htmlFor="password">Password</Label>
          <Link
            to="/forgot-password"
            className="text-xs text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
          >
            Forgot password?
          </Link>
        </div>
        <Input
          ref={passwordRef}
          id="password"
          name="password"
          type="password"
          autoComplete="current-password"
          placeholder="••••••••"
          minLength={SECURITY_POLICY.passwordMinLength}
          maxLength={SECURITY_POLICY.passwordMaxLength}
          required
          aria-invalid={!!error}
          aria-describedby={error ? "login-error" : undefined}
        />
      </div>
      {error && (
        <p id="login-error" role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <Button type="submit" size="lg" className="w-full" disabled={pending}>
        {pending ? "Signing in…" : "Sign in"}
      </Button>
    </form>
  )
}

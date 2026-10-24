import { useState, type FormEvent } from "react"
import { SECURITY_POLICY } from "@app/contracts"
import { createFileRoute, Link } from "@tanstack/react-router"
import { Button } from "@app/ui/components/button"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import { AuthLayout } from "@/features/auth/components/auth-layout"
import { resetPassword } from "@/lib/auth-api"
import { ApiError } from "@/lib/api-client"

export const Route = createFileRoute("/reset-password")({
  validateSearch: (search): { token?: string } =>
    typeof search.token === "string" ? { token: search.token } : {},
  component: ResetPasswordPage,
})

export function ResetPasswordPage() {
  const { token } = Route.useSearch()
  const [done, setDone] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!token) return
    const form = new FormData(e.currentTarget)
    setError(null)
    setPending(true)
    try {
      await resetPassword({
        channel: "web",
        token,
        newPassword: String(form.get("password")),
      })
      setDone(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Reset failed")
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthLayout
      title="Reset password"
      description="Choose a new password for your account."
    >
      {!token ? (
        <div className="space-y-4 text-sm">
          <p>Reset token is missing.</p>
          <Button asChild size="lg" className="w-full">
            <Link to="/forgot-password">Request a new link</Link>
          </Button>
        </div>
      ) : done ? (
        <div className="space-y-4 text-sm">
          <p role="status">Your password has been reset.</p>
          <Button asChild size="lg" className="w-full">
            <Link to="/login">Back to sign in</Link>
          </Button>
        </div>
      ) : (
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="password">New password</Label>
            <Input
              id="password"
              name="password"
              type="password"
              autoComplete="new-password"
              placeholder={`At least ${SECURITY_POLICY.passwordMinLength} characters`}
              minLength={SECURITY_POLICY.passwordMinLength}
              maxLength={SECURITY_POLICY.passwordMaxLength}
              required
            />
          </div>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <Button type="submit" size="lg" className="w-full" disabled={pending}>
            {pending ? "Resetting…" : "Reset password"}
          </Button>
        </form>
      )}
    </AuthLayout>
  )
}

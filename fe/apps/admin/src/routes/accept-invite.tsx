import { useState, type FormEvent } from "react"
import { SECURITY_POLICY } from "@app/contracts"
import { createFileRoute, Link } from "@tanstack/react-router"
import { Button } from "@app/ui/components/button"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import { AuthLayout } from "@/features/auth/components/auth-layout"
import { acceptInvite } from "@/lib/auth-api"
import { ApiError } from "@/lib/api-client"

export const Route = createFileRoute("/accept-invite")({
  validateSearch: (search): { token?: string } =>
    typeof search.token === "string" ? { token: search.token } : {},
  component: AcceptInvitePage,
})

function AcceptInvitePage() {
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
      await acceptInvite({
        token,
        fullName: String(form.get("fullName")),
        password: String(form.get("password")),
      })
      setDone(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Invite failed")
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthLayout
      title="Accept your invite"
      description="Set your name and password to activate your account."
    >
      {!token ? (
        <p className="text-sm">Invite token is missing.</p>
      ) : done ? (
        <div className="space-y-4 text-sm">
          <p role="status">Your account is ready.</p>
          <Button asChild size="lg" className="w-full">
            <Link to="/login">Sign in</Link>
          </Button>
        </div>
      ) : (
        <form onSubmit={onSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="fullName">Full name</Label>
            <Input id="fullName" name="fullName" autoComplete="name" required />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="password">Password</Label>
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
            {pending ? "Saving…" : "Activate account"}
          </Button>
        </form>
      )}
    </AuthLayout>
  )
}

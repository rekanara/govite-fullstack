import { useState, type FormEvent } from "react"
import { createFileRoute, Link } from "@tanstack/react-router"
import { MailCheckIcon } from "lucide-react"
import { formatDuration, SECURITY_POLICY } from "@app/contracts"
import { Button } from "@app/ui/components/button"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import { AuthLayout } from "@/features/auth/components/auth-layout"
import { forgotPassword } from "@/lib/auth-api"
import { ApiError } from "@/lib/api-client"

export const Route = createFileRoute("/forgot-password")({
  component: ForgotPasswordPage,
})

function ForgotPasswordPage() {
  const [sentTo, setSentTo] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const email = String(new FormData(e.currentTarget).get("email"))
    setError(null)
    setPending(true)
    try {
      await forgotPassword({ email, channel: "web" })
      setSentTo(email)
    } catch (err) {
      // The API answers the same for every email; only rate limiting is shown.
      if (err instanceof ApiError && err.status === 429)
        setError("Too many attempts. Try again in a minute.")
      else setSentTo(email)
    } finally {
      setPending(false)
    }
  }

  return (
    <AuthLayout
      title={sentTo ? "Check your email" : "Forgot your password?"}
      description={
        sentTo
          ? "Follow the link in the email to choose a new password."
          : "Enter your email and we'll send you a link to reset it."
      }
      footer={
        <p className="text-center text-sm">
          <Link
            to="/login"
            className="text-muted-foreground underline-offset-4 hover:text-primary hover:underline"
          >
            Back to sign in
          </Link>
        </p>
      }
    >
      {sentTo ? (
        <div className="space-y-4">
          <div
            role="status"
            className="flex items-start gap-3 rounded-lg border bg-muted/40 p-4 text-sm"
          >
            <MailCheckIcon className="mt-0.5 size-5 shrink-0 text-primary" />
            <p>
              If <span className="font-medium">{sentTo}</span> has an account,
              we've sent a reset link. It expires in{" "}
              {formatDuration(SECURITY_POLICY.passwordResetLinkTtlMs)}.
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="lg"
            className="w-full"
            onClick={() => setSentTo(null)}
          >
            Use a different email
          </Button>
        </div>
      ) : (
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
              aria-invalid={!!error}
              aria-describedby={error ? "forgot-error" : undefined}
            />
          </div>
          {error && (
            <p
              id="forgot-error"
              role="alert"
              className="text-sm text-destructive"
            >
              {error}
            </p>
          )}
          <Button type="submit" size="lg" className="w-full" disabled={pending}>
            {pending ? "Sending…" : "Send reset link"}
          </Button>
        </form>
      )}
    </AuthLayout>
  )
}

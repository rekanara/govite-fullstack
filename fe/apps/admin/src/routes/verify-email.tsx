import { useEffect, useState } from "react"
import { createFileRoute, Link } from "@tanstack/react-router"
import { Button } from "@app/ui/components/button"
import { AuthLayout } from "@/features/auth/components/auth-layout"
import { verifyEmail } from "@/lib/auth-api"

export const Route = createFileRoute("/verify-email")({
  validateSearch: (search): { token?: string } =>
    typeof search.token === "string" ? { token: search.token } : {},
  component: VerifyEmailPage,
})

function VerifyEmailPage() {
  const { token } = Route.useSearch()
  const [state, setState] = useState<"pending" | "done" | "failed">(
    token ? "pending" : "failed"
  )

  useEffect(() => {
    if (!token) return
    verifyEmail({ token }).then(
      () => setState("done"),
      () => setState("failed")
    )
  }, [token])

  return (
    <AuthLayout
      title="Verify email"
      description="Confirming your email address."
    >
      <div className="space-y-4 text-sm">
        <p role="status">
          {state === "pending"
            ? "Verifying…"
            : state === "done"
              ? "Your email is verified."
              : "This link is invalid or has expired."}
        </p>
        {state !== "pending" && (
          <Button asChild size="lg" className="w-full">
            <Link to="/admin/dashboard">Continue</Link>
          </Button>
        )}
      </div>
    </AuthLayout>
  )
}

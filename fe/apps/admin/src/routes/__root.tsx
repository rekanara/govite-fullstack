import type { ReactNode } from "react"
import {
  createRootRouteWithContext,
  Link,
  Outlet,
} from "@tanstack/react-router"
import { Button } from "@app/ui/components/button"
import { Toaster } from "@app/ui/components/sonner"
import type { RouterContext } from "@/router-context"
import { useTheme } from "@/components/theme-provider"

function Fallback({
  title,
  detail,
  action,
}: {
  title: string
  detail: string
  action: ReactNode
}) {
  return (
    <main className="grid min-h-svh place-items-center p-6 text-center">
      <div className="grid gap-3">
        <h1 className="text-2xl font-semibold">{title}</h1>
        <p className="text-sm text-muted-foreground">{detail}</p>
        <div>{action}</div>
      </div>
    </main>
  )
}

function RootLayout() {
  const { theme } = useTheme()
  return (
    <>
      <Outlet />
      <Toaster theme={theme} richColors position="top-right" />
    </>
  )
}

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  notFoundComponent: () => (
    <Fallback
      title="Page not found"
      detail="The page you are looking for does not exist."
      action={
        <Button asChild>
          <Link to="/">Go home</Link>
        </Button>
      }
    />
  ),
  errorComponent: ({ error, reset }) => (
    <Fallback
      title="Something went wrong"
      detail={error instanceof Error ? error.message : "Unexpected error"}
      action={<Button onClick={reset}>Try again</Button>}
    />
  ),
})

import { Fragment } from "react"
import {
  createFileRoute,
  Outlet,
  redirect,
  useLocation,
} from "@tanstack/react-router"
import { MoonIcon, SunIcon } from "lucide-react"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@app/ui/components/breadcrumb"
import { Button } from "@app/ui/components/button"
import { Separator } from "@app/ui/components/separator"
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@app/ui/components/sidebar"
import { TooltipProvider } from "@app/ui/components/tooltip"
import { AppSidebar } from "@/features/shell/app-sidebar"
import { NotificationBell } from "@/features/shell/notification-bell"
import { useTheme } from "@/components/theme-provider"
import { settledStatus } from "@/lib/auth-context"

export const Route = createFileRoute("/admin")({
  beforeLoad: async ({ context, location }) => {
    if ((await settledStatus(context.auth)) !== "authenticated") {
      throw redirect({ to: "/login", search: { redirect: location.href } })
    }
  },
  component: AdminLayout,
})

const UUID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const label = (segment: string) =>
  UUID_RE.test(segment)
    ? "Detail"
    : segment.charAt(0).toUpperCase() + segment.slice(1).replace(/-/g, " ")

function AdminLayout() {
  const segments = useLocation({
    select: (l) => l.pathname.split("/").filter(Boolean).slice(1),
  })
  const { theme, setTheme } = useTheme()
  const dark =
    theme === "dark" ||
    (theme === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches)

  return (
    <TooltipProvider>
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>
          <header className="flex h-14 shrink-0 items-center gap-2 px-4">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 h-4" />
            <Breadcrumb>
              <BreadcrumbList>
                {segments.map((segment, i) => (
                  <Fragment key={segment}>
                    {i > 0 && <BreadcrumbSeparator />}
                    <BreadcrumbItem>
                      {i === segments.length - 1 ? (
                        <BreadcrumbPage>{label(segment)}</BreadcrumbPage>
                      ) : (
                        label(segment)
                      )}
                    </BreadcrumbItem>
                  </Fragment>
                ))}
              </BreadcrumbList>
            </Breadcrumb>
            <div className="ml-auto" />
            <NotificationBell />
            <Button
              variant="ghost"
              size="icon"
              aria-label="Toggle theme"
              onClick={() => setTheme(dark ? "light" : "dark")}
            >
              {dark ? <SunIcon /> : <MoonIcon />}
            </Button>
          </header>
          <div className="flex flex-1 flex-col gap-4 p-4 pt-0">
            <Outlet />
          </div>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}

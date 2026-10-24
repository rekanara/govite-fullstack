import { Link } from "@tanstack/react-router"
import {
  KeyRoundIcon,
  LayoutDashboardIcon,
  MegaphoneIcon,
  ActivityIcon,
  ScrollTextIcon,
  SettingsIcon,
  ShieldIcon,
  UsersIcon,
  TerminalIcon,
  WebhookIcon,
} from "lucide-react"
import type { Resource, Action } from "@app/contracts"
import { NavMain, type NavItem } from "@/features/shell/nav-main"
import { NavUser } from "@/features/shell/nav-user"
import { useAuth } from "@/lib/auth-context"
import { usePublicSettings } from "@/lib/public-settings"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@app/ui/components/sidebar"

type Guarded = NavItem & { permission?: [Resource, Action] }

const overview: Guarded[] = [
  {
    title: "Dashboard",
    url: "/admin/dashboard",
    icon: <LayoutDashboardIcon />,
  },
]

const administration: Guarded[] = [
  {
    title: "Users",
    url: "/admin/administration/users",
    icon: <UsersIcon />,
    permission: ["users", "manage"],
  },
  {
    title: "Announcements",
    url: "/admin/administration/announcements",
    icon: <MegaphoneIcon />,
    permission: ["announcements", "manage"],
  },
  {
    title: "Roles",
    url: "/admin/administration/roles",
    icon: <ShieldIcon />,
    permission: ["roles", "manage"],
  },
  {
    title: "Permissions",
    url: "/admin/administration/permissions",
    icon: <KeyRoundIcon />,
    permission: ["permissions", "view"],
  },
  {
    title: "Audit log",
    url: "/admin/administration/audit",
    icon: <ScrollTextIcon />,
    permission: ["audit", "view"],
  },
  {
    title: "Webhooks",
    url: "/admin/administration/webhooks",
    icon: <WebhookIcon />,
    permission: ["webhooks", "manage"],
  },
  {
    title: "API keys",
    url: "/admin/administration/api-keys",
    icon: <KeyRoundIcon />,
    permission: ["apiKeys", "manage"],
  },
  {
    title: "Settings",
    url: "/admin/administration/settings",
    icon: <SettingsIcon />,
    permission: ["settings", "manage"],
  },
  {
    title: "System status",
    url: "/admin/administration/system",
    icon: <ActivityIcon />,
    permission: ["settings", "manage"],
  },
]

export function AppSidebar(props: React.ComponentProps<typeof Sidebar>) {
  const { can } = useAuth()
  const settings = usePublicSettings()
  const appName = settings.data?.appName ?? "Rekanara"
  const logoUrl = settings.data?.logoUrl ?? null
  const visible = (items: Guarded[]) =>
    items.filter((i) => !i.permission || can(...i.permission))
  const adminItems = visible(administration)

  return (
    <Sidebar variant="inset" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <Link to="/admin/dashboard">
                <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                  {logoUrl ? (
                    <img
                      src={logoUrl}
                      alt=""
                      className="size-5 object-contain"
                    />
                  ) : (
                    <TerminalIcon className="size-4" />
                  )}
                </div>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-medium">{appName}</span>
                  <span className="truncate text-xs">Admin</span>
                </div>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain label="Overview" items={visible(overview)} />
        {adminItems.length > 0 && (
          <NavMain label="Administration" items={adminItems} />
        )}
      </SidebarContent>
      <SidebarFooter>
        <NavUser />
      </SidebarFooter>
    </Sidebar>
  )
}

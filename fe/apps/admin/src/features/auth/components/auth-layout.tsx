import type { ReactNode } from "react"
import {
  BellRingIcon,
  ScrollTextIcon,
  ShieldCheckIcon,
  TerminalIcon,
  UsersIcon,
} from "lucide-react"
import { usePublicSettings } from "@/lib/public-settings"

const features = [
  {
    icon: UsersIcon,
    title: "Users & invitations",
    description: "Invite teammates by email and manage every account.",
  },
  {
    icon: ShieldCheckIcon,
    title: "Role-based access",
    description: "Each user only sees what their role allows.",
  },
  {
    icon: ScrollTextIcon,
    title: "Audit trail",
    description: "Every important change is recorded and traceable.",
  },
  {
    icon: BellRingIcon,
    title: "Notifications",
    description: "Stay informed on web and mobile in real time.",
  },
]

function Brand({
  inverted = false,
  appName,
  logoUrl,
}: {
  inverted?: boolean
  appName: string
  logoUrl: string | null
}) {
  return (
    <div className="flex items-center gap-2">
      <div
        className={
          inverted
            ? "flex size-9 items-center justify-center rounded-lg bg-primary-foreground/15"
            : "flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground"
        }
      >
        {logoUrl ? (
          <img src={logoUrl} alt="" className="size-6 object-contain" />
        ) : (
          <TerminalIcon className="size-5" />
        )}
      </div>
      <span className="text-lg font-semibold tracking-tight">{appName}</span>
    </div>
  )
}

export function AuthLayout({
  title,
  description,
  children,
  footer,
}: {
  title: string
  description: string
  children: ReactNode
  footer?: ReactNode
}) {
  const settings = usePublicSettings()
  const appName = settings.data?.appName ?? "Rekanara"
  const logoUrl = settings.data?.logoUrl ?? null
  const loginNotice = settings.data?.loginNotice
  const supportEmail = settings.data?.supportEmail

  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      <aside className="relative hidden flex-col justify-between overflow-hidden bg-primary p-10 text-primary-foreground lg:flex">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,rgb(255_255_255/0.15),transparent_55%)]" />
        <div className="relative">
          <Brand inverted appName={appName} logoUrl={logoUrl} />
        </div>
        <div className="relative flex flex-col gap-8">
          <div className="space-y-3">
            <h1 className="text-3xl font-bold tracking-tight text-balance">
              Everything you need to run your admin, in one place.
            </h1>
            <p className="max-w-md text-sm text-primary-foreground/80">
              Secure accounts, permissions and activity history, ready from day
              one.
            </p>
          </div>
          <ul className="flex flex-col gap-4">
            {features.map((feature) => (
              <li key={feature.title} className="flex items-start gap-3">
                <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary-foreground/15">
                  <feature.icon className="size-4" />
                </div>
                <div>
                  <p className="text-sm font-medium">{feature.title}</p>
                  <p className="text-sm text-primary-foreground/70">
                    {feature.description}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        </div>
        <p className="relative text-xs text-primary-foreground/60">
          © {new Date().getFullYear()} {appName}. All rights reserved.
        </p>
      </aside>

      <main className="flex flex-col items-center justify-center gap-8 p-6 sm:p-10">
        <div className="lg:hidden">
          <Brand appName={appName} logoUrl={logoUrl} />
        </div>
        <div className="w-full max-w-sm space-y-6">
          <div className="space-y-1.5 text-center sm:text-left">
            <h2 className="text-xl font-semibold tracking-tight">{title}</h2>
            <p className="text-sm text-muted-foreground">{description}</p>
          </div>
          {loginNotice && (
            <p className="rounded-lg border bg-muted/40 p-3 text-sm text-muted-foreground">
              {loginNotice}
            </p>
          )}
          {children}
          {supportEmail && (
            <p className="text-center text-xs text-muted-foreground">
              Need help? Contact{" "}
              <a
                href={`mailto:${supportEmail}`}
                className="underline-offset-4 hover:text-primary hover:underline"
              >
                {supportEmail}
              </a>
              .
            </p>
          )}
          {footer}
        </div>
      </main>
    </div>
  )
}

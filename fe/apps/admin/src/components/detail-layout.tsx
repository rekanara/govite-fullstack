import type { ReactNode } from "react"
import { Badge } from "@app/ui/components/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@app/ui/components/card"

export function DetailHero({
  eyebrow,
  title,
  description,
  badges,
  actions,
}: {
  eyebrow?: string
  title: ReactNode
  description?: ReactNode
  badges?: ReactNode
  actions?: ReactNode
}) {
  return (
    <div className="relative overflow-hidden rounded-3xl border bg-gradient-to-br from-card via-card to-muted/60 p-6 shadow-sm">
      <div className="absolute top-0 right-0 h-32 w-32 rounded-full bg-primary/10 blur-3xl" />
      <div className="relative flex flex-col gap-5 md:flex-row md:items-start md:justify-between">
        <div className="grid gap-3">
          {eyebrow && (
            <div className="text-xs font-medium tracking-[0.2em] text-muted-foreground uppercase">
              {eyebrow}
            </div>
          )}
          <div className="grid gap-2">
            <h1 className="text-3xl font-semibold tracking-tight md:text-4xl">
              {title}
            </h1>
            {description && (
              <p className="max-w-2xl text-sm text-muted-foreground">
                {description}
              </p>
            )}
          </div>
          {badges && <div className="flex flex-wrap gap-2">{badges}</div>}
        </div>
        {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
      </div>
    </div>
  )
}

export function DetailGrid({ children }: { children: ReactNode }) {
  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,420px)_minmax(0,1fr)]">
      {children}
    </div>
  )
}

export function DetailSection({
  title,
  description,
  actions,
  children,
}: {
  title: string
  description?: string
  actions?: ReactNode
  children: ReactNode
}) {
  return (
    <Card className="rounded-2xl shadow-sm">
      <CardHeader className="border-b">
        <div className="flex items-start justify-between gap-4">
          <div>
            <CardTitle className="text-base">{title}</CardTitle>
            {description && <CardDescription>{description}</CardDescription>}
          </div>
          {actions}
        </div>
      </CardHeader>
      <CardContent className="grid gap-4 pt-4">{children}</CardContent>
    </Card>
  )
}

export function DetailField({
  label,
  value,
}: {
  label: string
  value: ReactNode
}) {
  return (
    <div className="rounded-xl border bg-muted/20 p-3">
      <div className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
        {label}
      </div>
      <div className="mt-1 text-sm font-medium break-words">{value}</div>
    </div>
  )
}

export function DangerZone({ children }: { children: ReactNode }) {
  return (
    <Card className="rounded-2xl border-destructive/30 bg-destructive/5 shadow-sm">
      <CardHeader className="border-b border-destructive/20">
        <CardTitle className="text-base text-destructive">
          Danger zone
        </CardTitle>
        <CardDescription>
          High-impact actions require confirmation.
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3 pt-4">{children}</CardContent>
    </Card>
  )
}

export function StatusBadge({ active }: { active: boolean }) {
  return (
    <Badge variant={active ? "default" : "secondary"}>
      {active ? "Active" : "Inactive"}
    </Badge>
  )
}

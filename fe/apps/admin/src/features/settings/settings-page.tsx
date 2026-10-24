import { useState, type FormEvent } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { formatDuration, SECURITY_POLICY } from "@app/contracts"
import { Button } from "@app/ui/components/button"
import { Checkbox } from "@app/ui/components/checkbox"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@app/ui/components/card"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import { PageHeader } from "@/components/page"
import { RequirePermission } from "@/features/auth/require-permission"
import { settingsApi } from "@/features/settings/api"

export function SettingsPage() {
  const queryClient = useQueryClient()
  const settings = useQuery({
    queryKey: ["settings"],
    queryFn: settingsApi.get,
  })

  // undefined = keep; null = cleared; string = newly uploaded.
  const [logoUrl, setLogoUrl] = useState<string | null | undefined>(undefined)
  const [bannerUrl, setBannerUrl] = useState<string | null | undefined>(
    undefined
  )

  const upload = useMutation({
    mutationFn: settingsApi.uploadImage,
  })

  const save = useMutation({
    mutationFn: settingsApi.update,
    onSuccess: async () => {
      toast.success("Settings saved")
      setLogoUrl(undefined)
      setBannerUrl(undefined)
      await queryClient.invalidateQueries({ queryKey: ["settings"] })
    },
  })

  function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    const num = (v: FormDataEntryValue | null) => {
      const s = String(v ?? "").trim()
      return s ? Number(s) : null
    }
    const str = (v: FormDataEntryValue | null) => {
      const s = String(v ?? "").trim()
      return s || null
    }

    save.mutate({
      appName: String(form.get("appName")),
      tagline: str(form.get("tagline")),
      description: str(form.get("description")),
      supportEmail: str(form.get("supportEmail")),
      supportPhone: str(form.get("supportPhone")),
      logoUrl:
        logoUrl === undefined ? (settings.data?.logoUrl ?? null) : logoUrl,
      bannerUrl:
        bannerUrl === undefined
          ? (settings.data?.bannerUrl ?? null)
          : bannerUrl,
      loginNotice: str(form.get("loginNotice")),
      emailFooter: str(form.get("emailFooter")),
      timezone: String(form.get("timezone")) || "Asia/Jakarta",
      currency: (String(form.get("currency")) || "IDR").toUpperCase(),
      address: str(form.get("address")),
      district: str(form.get("district")),
      city: str(form.get("city")),
      province: str(form.get("province")),
      country: str(form.get("country")),
      postalCode: str(form.get("postalCode")),
      latitude: num(form.get("latitude")),
      longitude: num(form.get("longitude")),
      maintenanceRetentionDays: Number(form.get("maintenanceRetentionDays")),
      pushNotificationsEnabled: form.get("pushNotificationsEnabled") === "on",
    })
  }

  const currentLogo = logoUrl === undefined ? settings.data?.logoUrl : logoUrl
  const currentBanner =
    bannerUrl === undefined ? settings.data?.bannerUrl : bannerUrl

  return (
    <RequirePermission resource="settings" action="manage">
      <PageHeader title="Settings" description="Application-wide settings." />
      {settings.data && (
        <form
          onSubmit={onSubmit}
          className="grid items-start gap-6 lg:grid-cols-2"
        >
          {/* Left: General & Branding */}
          <Card>
            <CardHeader>
              <CardTitle>Branding & General</CardTitle>
              <CardDescription>
                App identity, images and basic preferences.
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-4">
              <div className="grid gap-2">
                <Label htmlFor="appName">App name</Label>
                <Input
                  id="appName"
                  name="appName"
                  defaultValue={settings.data.appName}
                  required
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="tagline">Tagline</Label>
                <Input
                  id="tagline"
                  name="tagline"
                  maxLength={150}
                  defaultValue={settings.data.tagline ?? ""}
                  placeholder="e.g. Modern Fullstack Starter"
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="description">Description</Label>
                <textarea
                  id="description"
                  name="description"
                  rows={3}
                  maxLength={1000}
                  defaultValue={settings.data.description ?? ""}
                  placeholder="Short description of the app"
                  className="flex w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs placeholder:text-muted-foreground focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
                />
              </div>

              <ImageUploader
                label="Logo"
                value={currentLogo}
                onSelect={(file) =>
                  upload.mutate(file, {
                    onSuccess: ({ url }) => {
                      setLogoUrl(url)
                      toast.success("Logo uploaded; save to apply")
                    },
                  })
                }
                onClear={() => setLogoUrl(null)}
                pending={upload.isPending}
                aspect="square"
              />

              <ImageUploader
                label="Banner"
                value={currentBanner}
                onSelect={(file) =>
                  upload.mutate(file, {
                    onSuccess: ({ url }) => {
                      setBannerUrl(url)
                      toast.success("Banner uploaded; save to apply")
                    },
                  })
                }
                onClear={() => setBannerUrl(null)}
                pending={upload.isPending}
                aspect="wide"
              />

              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-2">
                  <Label htmlFor="currency">Currency</Label>
                  <Input
                    id="currency"
                    name="currency"
                    maxLength={3}
                    defaultValue={settings.data.currency}
                    placeholder="IDR"
                    required
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="timezone">Timezone</Label>
                  <Input
                    id="timezone"
                    name="timezone"
                    defaultValue={settings.data.timezone}
                    required
                  />
                </div>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="maintenanceRetentionDays">
                  Cleanup retention (days)
                </Label>
                <Input
                  id="maintenanceRetentionDays"
                  name="maintenanceRetentionDays"
                  type="number"
                  min={7}
                  max={3650}
                  defaultValue={settings.data.maintenanceRetentionDays}
                  required
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="loginNotice">Login notice</Label>
                <Input
                  id="loginNotice"
                  name="loginNotice"
                  maxLength={500}
                  defaultValue={settings.data.loginNotice ?? ""}
                  placeholder="Optional message shown on the sign-in page"
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="emailFooter">Email footer</Label>
                <Input
                  id="emailFooter"
                  name="emailFooter"
                  maxLength={1000}
                  defaultValue={settings.data.emailFooter ?? ""}
                  placeholder="Optional footer appended to transactional emails"
                />
              </div>

              <label className="flex items-center gap-3 text-sm">
                <Checkbox
                  name="pushNotificationsEnabled"
                  defaultChecked={settings.data.pushNotificationsEnabled}
                />
                Send push notifications
              </label>

              <Button className="w-fit" disabled={save.isPending}>
                Save settings
              </Button>
            </CardContent>
          </Card>

          {/* Right column: Contact & Location, then Security */}
          <div className="grid gap-6">
            <Card>
              <CardHeader>
                <CardTitle>Contact & Location</CardTitle>
                <CardDescription>
                  Support contact and operating address.
                </CardDescription>
              </CardHeader>
              <CardContent className="grid gap-4">
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <div className="grid gap-2">
                    <Label htmlFor="supportEmail">Support email</Label>
                    <Input
                      id="supportEmail"
                      name="supportEmail"
                      type="email"
                      defaultValue={settings.data.supportEmail ?? ""}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="supportPhone">Support phone</Label>
                    <Input
                      id="supportPhone"
                      name="supportPhone"
                      type="tel"
                      maxLength={30}
                      defaultValue={settings.data.supportPhone ?? ""}
                      placeholder="+62 812..."
                    />
                  </div>
                </div>

                <div className="grid gap-2">
                  <Label htmlFor="address">Address</Label>
                  <Input
                    id="address"
                    name="address"
                    maxLength={200}
                    defaultValue={settings.data.address ?? ""}
                    placeholder="Jalan, nomor, gedung..."
                  />
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div className="grid gap-2">
                    <Label htmlFor="district">District</Label>
                    <Input
                      id="district"
                      name="district"
                      maxLength={100}
                      defaultValue={settings.data.district ?? ""}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="city">City</Label>
                    <Input
                      id="city"
                      name="city"
                      maxLength={100}
                      defaultValue={settings.data.city ?? ""}
                    />
                  </div>
                </div>

                <div className="grid grid-cols-3 gap-3">
                  <div className="grid gap-2">
                    <Label htmlFor="province">Province</Label>
                    <Input
                      id="province"
                      name="province"
                      maxLength={100}
                      defaultValue={settings.data.province ?? ""}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="country">Country</Label>
                    <Input
                      id="country"
                      name="country"
                      maxLength={100}
                      defaultValue={settings.data.country ?? ""}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="postalCode">Postal code</Label>
                    <Input
                      id="postalCode"
                      name="postalCode"
                      maxLength={20}
                      defaultValue={settings.data.postalCode ?? ""}
                    />
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div className="grid gap-2">
                    <Label htmlFor="latitude">Latitude</Label>
                    <Input
                      id="latitude"
                      name="latitude"
                      type="number"
                      step="any"
                      min={-90}
                      max={90}
                      defaultValue={settings.data.latitude ?? ""}
                      placeholder="-6.200000"
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="longitude">Longitude</Label>
                    <Input
                      id="longitude"
                      name="longitude"
                      type="number"
                      step="any"
                      min={-180}
                      max={180}
                      defaultValue={settings.data.longitude ?? ""}
                      placeholder="106.816666"
                    />
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Security</CardTitle>
                <CardDescription>
                  Fixed limits enforced by the API. Change them in code, not
                  here.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <dl className="grid gap-3 text-sm sm:grid-cols-2">
                  {(
                    [
                      [
                        "Password length",
                        `${SECURITY_POLICY.passwordMinLength}–${SECURITY_POLICY.passwordMaxLength} characters`,
                      ],
                      [
                        "Access token",
                        formatDuration(SECURITY_POLICY.accessTokenTtlMs),
                      ],
                      [
                        "Session lifetime",
                        `${formatDuration(SECURITY_POLICY.refreshTokenTtlMs)} per sign-in, renewed on use`,
                      ],
                      [
                        "Password reset link",
                        formatDuration(SECURITY_POLICY.passwordResetLinkTtlMs),
                      ],
                      [
                        "Password reset code",
                        formatDuration(SECURITY_POLICY.passwordResetCodeTtlMs),
                      ],
                      [
                        "Invite link",
                        formatDuration(SECURITY_POLICY.inviteTtlMs),
                      ],
                      [
                        "Email verification link",
                        formatDuration(SECURITY_POLICY.emailVerificationTtlMs),
                      ],
                      [
                        "Wrong code attempts",
                        `${SECURITY_POLICY.maxCodeAttempts} per code`,
                      ],
                    ] as const
                  ).map(([label, value]) => (
                    <div key={label}>
                      <dt className="text-muted-foreground">{label}</dt>
                      <dd className="font-medium">{value}</dd>
                    </div>
                  ))}
                </dl>
              </CardContent>
            </Card>
          </div>
        </form>
      )}
    </RequirePermission>
  )
}

function ImageUploader({
  label,
  value,
  onSelect,
  onClear,
  pending,
  aspect,
}: {
  label: string
  value?: string | null
  onSelect: (file: File) => void
  onClear: () => void
  pending: boolean
  aspect: "square" | "wide"
}) {
  return (
    <div className="grid gap-2">
      <Label>{label}</Label>
      <div className="flex items-center gap-3">
        {value && (
          <img
            src={value}
            alt=""
            className={
              aspect === "wide"
                ? "h-10 w-24 rounded-md border object-cover"
                : "size-10 rounded-md border object-contain"
            }
          />
        )}
        <Input
          type="file"
          accept="image/jpeg,image/png,image/webp"
          className="max-w-xs"
          disabled={pending}
          onChange={(e) => {
            const file = e.currentTarget.files?.[0]
            if (file) onSelect(file)
          }}
        />
        {value && (
          <Button type="button" variant="ghost" size="sm" onClick={onClear}>
            Clear
          </Button>
        )}
      </div>
    </div>
  )
}

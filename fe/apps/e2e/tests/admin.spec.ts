import { expect, test, type Page } from "@playwright/test"

// Needs a running API seeded with this user, whose role must not require 2FA
// (scripts/e2e.sh sets that up).
const email = process.env.SUPER_ADMIN_EMAIL ?? "smoke@app.local"
const password = process.env.SUPER_ADMIN_PASSWORD ?? "Smoke12345!"

// Every page moved into features/ during the refactor, with its heading.
const pages: [path: string, heading: string][] = [
  ["/admin/dashboard", "Dashboard"],
  ["/admin/administration/users", "Users"],
  ["/admin/administration/roles", "Roles"],
  ["/admin/administration/permissions", "Permissions"],
  ["/admin/administration/announcements", "Announcements"],
  ["/admin/administration/audit", "Audit log"],
  ["/admin/administration/settings", "Settings"],
  ["/admin/administration/system", "System"],
  ["/admin/administration/webhooks", "Webhooks"],
  ["/admin/administration/api-keys", "API keys"],
  ["/admin/notifications", "Notifications"],
  ["/admin/account", "Account"],
]

/** Collects console errors and failed API calls so a page that renders but
 *  breaks underneath still fails the test. */
function watch(page: Page) {
  const problems: string[] = []
  page.on("console", (m) => {
    // The browser logs every non-2xx response itself; those are judged by URL
    // in the response handler below instead.
    if (m.type() === "error" && !m.text().startsWith("Failed to load resource"))
      problems.push(`console: ${m.text()}`)
  })
  page.on("pageerror", (e) => problems.push(`pageerror: ${e.message}`))
  page.on("response", (r) => {
    const path = new URL(r.url()).pathname
    // A signed-out visit probes the session with POST /auth/refresh and gets
    // 401 by design; that is the only expected failure.
    if (path === "/api/v1/auth/refresh" && r.status() === 401) return
    if (path.startsWith("/api/") && r.status() >= 400)
      problems.push(`${r.status()} ${r.request().method()} ${path}`)
  })
  return problems
}

async function signIn(page: Page) {
  await page.goto("/login")
  await page.getByLabel("Email").fill(email)
  await page.getByLabel("Password").fill(password)
  await page.getByRole("button", { name: "Sign in" }).click()
  await expect(page).toHaveURL(/\/admin\/dashboard/)
}

test("sign in, session survives reload, every page renders, sign out", async ({
  page,
}) => {
  const problems = watch(page)
  await signIn(page)

  // The access token lives in memory; a reload must restore the session
  // through the refresh cookie (path /api/v1/auth, same origin via proxy).
  await page.reload()
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible()

  for (const [path, heading] of pages) {
    await page.goto(path)
    await expect(
      page.getByRole("heading", { level: 1, name: heading }),
      path
    ).toBeVisible()
    await expect(page.getByText("Access denied")).toHaveCount(0)
  }

  // The sidebar user button shows the role under the name.
  await page.getByRole("button", { name: /Super Admin/ }).click()
  await page.getByRole("menuitem", { name: "Log out" }).click()
  await expect(page).toHaveURL(/\/login/)
  await page.goto("/admin/dashboard")
  await expect(page).toHaveURL(/\/login/)

  expect(problems).toEqual([])
})

test("create a user from the dialog", async ({ page }) => {
  const problems = watch(page)
  await signIn(page)
  await page.goto("/admin/administration/users")

  const newEmail = `e2e-${Date.now()}@test.local`
  await page.getByRole("button", { name: "Add user" }).click()
  const dialog = page.getByRole("dialog", { name: "New user" })
  await dialog.getByLabel("Full name").fill("E2E User")
  await dialog.getByLabel("Email").fill(newEmail)
  await dialog.getByLabel("Initial password").fill(password)
  await dialog.getByRole("combobox", { name: "Role" }).click()
  await page.getByRole("option").first().click()
  await dialog.getByRole("button", { name: "Create" }).click()

  await expect(dialog).toBeHidden()
  await page.getByPlaceholder(/search/i).fill(newEmail)
  await expect(
    page.getByRole("cell", { name: newEmail, exact: true })
  ).toBeVisible()

  expect(problems).toEqual([])
})

import { afterEach, describe, expect, it, vi } from "vitest"
import { LoginForm } from "./login-form"
import { click, render, submit } from "../../../test-utils"
import { ApiError } from "../../../lib/api-client"

const login = vi.fn()
const verifyTwoFactor = vi.fn()
const verifyTwoFactorBackup = vi.fn()
const navigate = vi.fn()
const push = vi.fn()

vi.mock("@/lib/auth-context", () => ({
  useAuth: () => ({ login, verifyTwoFactor, verifyTwoFactorBackup }),
}))
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => (
    <a href={to}>{children}</a>
  ),
  useNavigate: () => navigate,
  useRouter: () => ({ history: { push } }),
}))

afterEach(() => {
  login.mockReset()
  verifyTwoFactor.mockReset()
  verifyTwoFactorBackup.mockReset()
  navigate.mockReset()
  push.mockReset()
  document.body.replaceChildren()
})

describe("LoginForm", () => {
  it("submits credentials and redirects", async () => {
    login.mockResolvedValue({ requiresTwoFactor: false })
    await render(<LoginForm redirectTo="/admin/users" />)
    ;(document.querySelector("#email") as HTMLInputElement).value =
      "a@test.local"
    ;(document.querySelector("#password") as HTMLInputElement).value =
      "Password123!"
    await submit(document.querySelector("form")!)
    expect(login).toHaveBeenCalledWith("a@test.local", "Password123!")
    expect(push).toHaveBeenCalledWith("/admin/users")
  })

  it("submits two-factor code after password challenge", async () => {
    login.mockResolvedValue({
      requiresTwoFactor: true,
      challengeToken: "challenge",
      setupRequired: true,
      manualKey: "SECRET",
      otpauthUrl: "otpauth://totp/App:a@test.local?secret=SECRET",
    })
    verifyTwoFactor.mockResolvedValue({
      requiresTwoFactor: false,
      backupCodes: ["11111-11111", "22222-22222"],
    })
    await render(<LoginForm redirectTo="/admin/users" />)
    ;(document.querySelector("#email") as HTMLInputElement).value =
      "a@test.local"
    ;(document.querySelector("#password") as HTMLInputElement).value =
      "Password123!"
    await submit(document.querySelector("form")!)
    expect(document.body.textContent).toContain("SECRET")
    expect(document.querySelector("svg")).not.toBeNull()
    const code = document.querySelector("#code") as HTMLInputElement
    expect(new RegExp(`^(?:${code.pattern})$`).test("123456")).toBe(true)
    code.value = "123456"
    await submit(document.querySelector("form")!)
    expect(verifyTwoFactor).toHaveBeenCalledWith("challenge", "123456")
    // Backup codes are shown once before the redirect happens.
    expect(push).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain("11111-11111")
    const continueBtn = [...document.querySelectorAll("button")].find((b) =>
      b.textContent?.includes("continue")
    )
    expect(continueBtn).not.toBeUndefined()
    await click(continueBtn!)
    expect(push).toHaveBeenCalledWith("/admin/users")
  })

  it("logs in with a backup code when authenticator is unavailable", async () => {
    login.mockResolvedValue({
      requiresTwoFactor: true,
      challengeToken: "challenge",
      setupRequired: false,
    })
    verifyTwoFactorBackup.mockResolvedValue({ requiresTwoFactor: false })
    await render(<LoginForm redirectTo="/admin/users" />)
    ;(document.querySelector("#email") as HTMLInputElement).value =
      "a@test.local"
    ;(document.querySelector("#password") as HTMLInputElement).value =
      "Password123!"
    await submit(document.querySelector("form")!)
    await click(document.querySelector("button[type=button]")!)
    ;(document.querySelector("#backupCode") as HTMLInputElement).value =
      "11111-11111"
    await submit(document.querySelector("form")!)
    expect(verifyTwoFactorBackup).toHaveBeenCalledWith(
      "challenge",
      "11111-11111"
    )
    expect(push).toHaveBeenCalledWith("/admin/users")
  })

  it("shows throttled error and clears password", async () => {
    login.mockRejectedValue(new ApiError(429, undefined, "Too many"))
    await render(<LoginForm />)
    const password = document.querySelector("#password") as HTMLInputElement
    ;(document.querySelector("#email") as HTMLInputElement).value =
      "a@test.local"
    password.value = "Password123!"
    await submit(document.querySelector("form")!)
    expect(document.querySelector('[role="alert"]')?.textContent).toContain(
      "Too many attempts"
    )
    expect(password.value).toBe("")
  })
})

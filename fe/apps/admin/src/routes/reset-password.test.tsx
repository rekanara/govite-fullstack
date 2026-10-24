import { afterEach, describe, expect, it, vi } from "vitest"
import { ResetPasswordPage } from "./reset-password"
import { render, submit } from "../test-utils"
import { ApiError } from "../lib/api-client"

const state = vi.hoisted(() => ({
  resetPassword: vi.fn(),
  token: "tok" as string | undefined,
}))

vi.mock("@/features/auth/components/auth-layout", () => ({
  AuthLayout: ({ children }: { children: React.ReactNode }) => (
    <main>{children}</main>
  ),
}))
vi.mock("@/lib/auth-api", () => ({ resetPassword: state.resetPassword }))
vi.mock("@tanstack/react-router", () => ({
  createFileRoute: () => (cfg: unknown) => ({
    ...(cfg as object),
    useSearch: () => ({ token: state.token }),
  }),
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => (
    <a href={to}>{children}</a>
  ),
}))

afterEach(() => {
  state.resetPassword.mockReset()
  state.token = "tok"
  document.body.replaceChildren()
})

describe("ResetPasswordPage", () => {
  it("submits token and new password", async () => {
    state.resetPassword.mockResolvedValue(undefined)
    await render(<ResetPasswordPage />)
    ;(document.querySelector("#password") as HTMLInputElement).value =
      "Password123!"
    await submit(document.querySelector("form")!)
    expect(state.resetPassword).toHaveBeenCalledWith({
      channel: "web",
      token: "tok",
      newPassword: "Password123!",
    })
    expect(document.querySelector('[role="status"]')?.textContent).toContain(
      "reset"
    )
  })

  it("handles missing token and API error", async () => {
    state.token = undefined
    await render(<ResetPasswordPage />)
    expect(document.body.textContent).toContain("Reset token is missing")
    document.body.replaceChildren()
    state.token = "tok"
    state.resetPassword.mockRejectedValue(new ApiError(400, "BAD", "Bad token"))
    await render(<ResetPasswordPage />)
    ;(document.querySelector("#password") as HTMLInputElement).value =
      "Password123!"
    await submit(document.querySelector("form")!)
    expect(document.querySelector('[role="alert"]')?.textContent).toBe(
      "Bad token"
    )
  })
})

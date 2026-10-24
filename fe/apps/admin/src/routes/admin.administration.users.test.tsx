import { afterEach, describe, expect, it, vi } from "vitest"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { UsersPage } from "./admin.administration.users"
import { render } from "../test-utils"

const state = vi.hoisted(() => ({ bulk: vi.fn() }))

vi.mock("@/components/page", () => ({
  PageHeader: ({
    title,
    actions,
  }: {
    title: string
    actions?: React.ReactNode
  }) => (
    <header>
      <h1>{title}</h1>
      {actions}
    </header>
  ),
  RequirePermission: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}))
vi.mock("@/features/admin/user-import-dialog", () => ({
  UserImportDialog: () => null,
}))
vi.mock("@tanstack/react-router", () => ({
  createFileRoute: () => (cfg: unknown) => cfg,
  Link: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
}))
vi.mock("@/features/admin/api", () => ({
  usersApi: {
    list: () =>
      Promise.resolve({
        data: [
          {
            id: "11111111-1111-4111-8111-111111111111",
            email: "one@test.local",
            fullName: "One",
            roleId: "role",
            roleName: "Admin",
            isActive: true,
            invited: false,
            deletedAt: null,
            emailVerifiedAt: null,
            lastLoginAt: null,
            avatarUrl: null,
            createdAt: new Date().toISOString(),
          },
        ],
        meta: { total: 1, page: 1, limit: 20, totalPages: 1 },
      }),
    bulk: state.bulk,
    export: vi.fn(),
    create: vi.fn(),
  },
  rolesApi: {
    list: () =>
      Promise.resolve({
        data: [],
        meta: { total: 0, page: 1, limit: 100, totalPages: 0 },
      }),
  },
}))

const renderPage = () =>
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <UsersPage />
    </QueryClientProvider>
  )

afterEach(() => {
  state.bulk.mockReset()
  document.body.replaceChildren()
})

describe("UsersPage bulk actions", () => {
  it("sends selected users to bulk deactivate", async () => {
    state.bulk.mockResolvedValue({
      succeeded: ["11111111-1111-4111-8111-111111111111"],
      failed: [],
    })
    await renderPage()
    await vi.waitFor(() =>
      expect(document.body.textContent).toContain("one@test.local")
    )
    ;(
      document.querySelector(
        '[aria-label="Select one@test.local"]'
      ) as HTMLButtonElement
    ).click()
    await vi.waitFor(() =>
      expect(document.body.textContent).toContain("1 selected")
    )
    ;[...document.querySelectorAll("button")]
      .find((b) => b.textContent === "Deactivate")!
      .click()
    await vi.waitFor(() =>
      expect(state.bulk).toHaveBeenCalledWith({
        ids: ["11111111-1111-4111-8111-111111111111"],
        action: "deactivate",
      })
    )
  })
})

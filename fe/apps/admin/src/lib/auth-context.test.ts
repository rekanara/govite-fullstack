import { describe, expect, it } from "vitest"
import { settledStatus } from "./auth-context"

describe("settledStatus", () => {
  it("waits for the boot probe while loading (browser refresh)", async () => {
    const ready = Promise.resolve("authenticated" as const)
    expect(await settledStatus({ status: "loading", ready })).toBe(
      "authenticated"
    )
  })

  it("uses live status after login even if boot probe said anonymous", async () => {
    const ready = Promise.resolve("anonymous" as const)
    expect(await settledStatus({ status: "authenticated", ready })).toBe(
      "authenticated"
    )
  })

  it("uses live status after logout", async () => {
    const ready = Promise.resolve("authenticated" as const)
    expect(await settledStatus({ status: "anonymous", ready })).toBe(
      "anonymous"
    )
  })
})

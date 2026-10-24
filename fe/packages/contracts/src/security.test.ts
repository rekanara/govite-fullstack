import { describe, expect, it } from "vitest"
import { formatDuration, SECURITY_POLICY } from "./security"

describe("formatDuration", () => {
  it("picks the largest whole unit", () => {
    expect(formatDuration(SECURITY_POLICY.passwordResetLinkTtlMs)).toBe(
      "30 minutes"
    )
    expect(formatDuration(SECURITY_POLICY.emailVerificationTtlMs)).toBe("1 day")
    expect(formatDuration(SECURITY_POLICY.inviteTtlMs)).toBe("7 days")
    expect(formatDuration(90 * 60 * 1000)).toBe("90 minutes")
  })
})

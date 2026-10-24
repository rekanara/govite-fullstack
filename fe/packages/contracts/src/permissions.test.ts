import { describe, expect, it } from "vitest"
import { isKnownPermission, permissionKey, PERMISSIONS } from "./permissions"

describe("permissions", () => {
  it("has unique keys", () => {
    const keys = PERMISSIONS.map(permissionKey)
    expect(new Set(keys).size).toBe(keys.length)
  })
  it("recognises known pairs only", () => {
    expect(isKnownPermission("roles", "manage")).toBe(true)
    expect(isKnownPermission("roles", "nope")).toBe(false)
  })
})

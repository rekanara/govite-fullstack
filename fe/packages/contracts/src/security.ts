import { z } from "zod"
import { SECURITY_POLICY } from "./generated"

const MINUTE = 60 * 1000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

// Generated from be/internal/policy (`make gen` in be/).
export { SECURITY_POLICY } from "./generated"

export const PasswordSchema = z
  .string()
  .min(SECURITY_POLICY.passwordMinLength)
  .max(SECURITY_POLICY.passwordMaxLength)

/** "30 minutes", "1 day", "7 days": for emails and the admin UI. */
export function formatDuration(ms: number): string {
  const units = [
    ["day", DAY],
    ["hour", HOUR],
    ["minute", MINUTE],
  ] as const
  for (const [name, size] of units) {
    if (ms >= size && ms % size === 0) {
      const n = ms / size
      return `${n} ${name}${n === 1 ? "" : "s"}`
    }
  }
  return `${Math.round(ms / 1000)} seconds`
}

import { z } from "zod"
import { PasswordSchema as password } from "./security"

export const LoginRequestSchema = z.object({
  email: z.email(),
  password,
})
export type LoginRequest = z.infer<typeof LoginRequestSchema>

export const LoginResponseSchema = z.discriminatedUnion("requiresTwoFactor", [
  z.object({
    requiresTwoFactor: z.literal(true),
    challengeToken: z.string(),
    setupRequired: z.boolean(),
    manualKey: z.string().optional(),
    otpauthUrl: z.string().optional(),
  }),
  z.object({
    requiresTwoFactor: z.literal(false).default(false),
    accessToken: z.string(),
    accessTokenExpiresInSeconds: z.number(),
    user: z.object({
      id: z.uuid(),
      email: z.email(),
      roleId: z.uuid(),
    }),
    refreshToken: z.string().optional(),
    // Only present right after first-time 2FA setup; shown once.
    backupCodes: z.array(z.string()).optional(),
  }),
])
export type LoginResponse = z.infer<typeof LoginResponseSchema>

export const TwoFactorVerifyRequestSchema = z.object({
  challengeToken: z.string().min(32).max(256),
  code: z.string().regex(/^\d{6}$/),
})
export type TwoFactorVerifyRequest = z.infer<
  typeof TwoFactorVerifyRequestSchema
>

export const TwoFactorBackupRequestSchema = z.object({
  challengeToken: z.string().min(32).max(256),
  backupCode: z.string().trim().min(1).max(32),
})
export type TwoFactorBackupRequest = z.infer<
  typeof TwoFactorBackupRequestSchema
>

export const RefreshResponseSchema = z.object({
  accessToken: z.string(),
  accessTokenExpiresInSeconds: z.number(),
  refreshToken: z.string().optional(),
})
export type RefreshResponse = z.infer<typeof RefreshResponseSchema>

export const ForgotPasswordRequestSchema = z.object({
  email: z.email(),
  channel: z.enum(["web", "mobile"]).default("web"),
})
export type ForgotPasswordRequest = z.infer<typeof ForgotPasswordRequestSchema>

export const ResetPasswordRequestSchema = z.discriminatedUnion("channel", [
  z.object({
    channel: z.literal("web"),
    token: z.string().min(32).max(256),
    newPassword: password,
  }),
  z.object({
    channel: z.literal("mobile"),
    email: z.email(),
    otp: z.string().regex(/^\d{6}$/),
    newPassword: password,
  }),
])
export type ResetPasswordRequest = z.infer<typeof ResetPasswordRequestSchema>

export const AcceptInviteRequestSchema = z.object({
  token: z.string().min(32).max(256),
  fullName: z.string().trim().min(1).max(100),
  password,
})
export type AcceptInviteRequest = z.infer<typeof AcceptInviteRequestSchema>

export const VerifyEmailRequestSchema = z.object({
  token: z.string().min(32).max(256),
})
export type VerifyEmailRequest = z.infer<typeof VerifyEmailRequestSchema>

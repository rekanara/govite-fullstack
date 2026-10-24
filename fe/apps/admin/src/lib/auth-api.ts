import {
  AcceptInviteRequestSchema,
  ForgotPasswordRequestSchema,
  LoginResponseSchema,
  MeResponseSchema,
  RefreshResponseSchema,
  ResetPasswordRequestSchema,
  TwoFactorBackupRequestSchema,
  TwoFactorVerifyRequestSchema,
  VerifyEmailRequestSchema,
  type AcceptInviteRequest,
  type ForgotPasswordRequest,
  type LoginRequest,
  type LoginResponse,
  type MeResponse,
  type RefreshResponse,
  type ResetPasswordRequest,
  type TwoFactorBackupRequest,
  type TwoFactorVerifyRequest,
  type VerifyEmailRequest,
} from "@app/contracts"
import { apiRequest } from "./api-client"

export function login(credentials: LoginRequest): Promise<LoginResponse> {
  return apiRequest("/auth/login", {
    method: "POST",
    body: credentials,
    schema: LoginResponseSchema,
  })
}

export function verifyTwoFactor(
  body: TwoFactorVerifyRequest
): Promise<LoginResponse> {
  TwoFactorVerifyRequestSchema.parse(body)
  return apiRequest("/auth/2fa/verify", {
    method: "POST",
    body,
    schema: LoginResponseSchema,
  })
}

export function verifyTwoFactorBackup(
  body: TwoFactorBackupRequest
): Promise<LoginResponse> {
  TwoFactorBackupRequestSchema.parse(body)
  return apiRequest("/auth/2fa/verify-backup", {
    method: "POST",
    body,
    schema: LoginResponseSchema,
  })
}

export function refreshAccessToken(): Promise<RefreshResponse> {
  return apiRequest("/auth/refresh", {
    method: "POST",
    schema: RefreshResponseSchema,
  })
}

export function getMe(accessToken: string): Promise<MeResponse> {
  return apiRequest("/auth/me", { accessToken, schema: MeResponseSchema })
}

export function logout(): Promise<void> {
  return apiRequest("/auth/logout", { method: "POST" })
}

export function forgotPassword(body: ForgotPasswordRequest): Promise<void> {
  ForgotPasswordRequestSchema.parse(body)
  return apiRequest("/auth/forgot-password", { method: "POST", body })
}

export function resetPassword(body: ResetPasswordRequest): Promise<void> {
  ResetPasswordRequestSchema.parse(body)
  return apiRequest("/auth/reset-password", { method: "POST", body })
}

export function acceptInvite(body: AcceptInviteRequest): Promise<void> {
  AcceptInviteRequestSchema.parse(body)
  return apiRequest("/auth/accept-invite", { method: "POST", body })
}

export function verifyEmail(body: VerifyEmailRequest): Promise<void> {
  VerifyEmailRequestSchema.parse(body)
  return apiRequest("/auth/verify-email", { method: "POST", body })
}

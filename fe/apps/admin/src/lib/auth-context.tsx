import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"
import { flushSync } from "react-dom"
import type { LoginResponse, MeResponse, Scope } from "@app/contracts"
import * as authApi from "./auth-api"
import { queryClient } from "./query-client"
import { refreshSession } from "./refresh-session"
import { setOnSessionExpired } from "./authed-request"
import { getAccessToken, setAccessToken } from "./token-store"

type SettledStatus = "authenticated" | "anonymous"
type AuthStatus = "loading" | SettledStatus

interface AuthState {
  status: AuthStatus
  user: MeResponse | null
}

export interface AuthContextValue extends AuthState {
  ready: Promise<SettledStatus>
  login: (email: string, password: string) => Promise<LoginResponse>
  verifyTwoFactor: (
    challengeToken: string,
    code: string
  ) => Promise<LoginResponse>
  verifyTwoFactorBackup: (
    challengeToken: string,
    backupCode: string
  ) => Promise<LoginResponse>
  logout: () => Promise<void>
  // Reloads /auth/me into state (after profile or photo changes); never throws.
  refreshUser: () => Promise<void>
  can: (resource: string, action: string) => boolean
  // Widest scope granted for the permission, or null when not granted.
  scopeOf: (resource: string, action: string) => Scope | null
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({
    status: "loading",
    user: null,
  })

  const [ready] = useState<{
    promise: Promise<SettledStatus>
    resolve: (status: SettledStatus) => void
  }>(() => {
    let resolve!: (status: SettledStatus) => void
    const promise = new Promise<SettledStatus>((res) => {
      resolve = res
    })
    return { promise, resolve }
  })

  useEffect(() => {
    let cancelled = false
    refreshSession().then((session) => {
      if (session) {
        setAccessToken(session.accessToken)
        if (!cancelled)
          setState({ status: "authenticated", user: session.user })
        ready.resolve("authenticated")
      } else {
        if (!cancelled) setState({ status: "anonymous", user: null })
        ready.resolve("anonymous")
      }
    })
    return () => {
      cancelled = true
    }
  }, [ready])

  useEffect(() => {
    setOnSessionExpired(() => {
      setAccessToken(null)
      queryClient.clear()
      setState({ status: "anonymous", user: null })
    })
    return () => setOnSessionExpired(null)
  }, [])

  const finishLogin = useCallback(async (result: LoginResponse) => {
    if (result.requiresTwoFactor) return result
    setAccessToken(result.accessToken)
    // Drop anything cached for a previous session before the new user renders.
    queryClient.clear()
    const user = await authApi.getMe(result.accessToken)
    // Synchronous so the router's context is updated before a caller's
    // post-login navigate() runs - otherwise beforeLoad reads the stale
    // pre-login snapshot and bounces straight back to /login.
    flushSync(() => setState({ status: "authenticated", user }))
    return result
  }, [])

  const login = useCallback(
    async (email: string, password: string) =>
      finishLogin(await authApi.login({ email, password })),
    [finishLogin]
  )

  const verifyTwoFactor = useCallback(
    async (challengeToken: string, code: string) =>
      finishLogin(await authApi.verifyTwoFactor({ challengeToken, code })),
    [finishLogin]
  )

  const verifyTwoFactorBackup = useCallback(
    async (challengeToken: string, backupCode: string) =>
      finishLogin(
        await authApi.verifyTwoFactorBackup({ challengeToken, backupCode })
      ),
    [finishLogin]
  )

  const logout = useCallback(async () => {
    await authApi.logout().catch(() => undefined)
    setAccessToken(null)
    flushSync(() => setState({ status: "anonymous", user: null }))
    // Clear only after the state flip has unmounted the admin pages: clearing
    // under mounted queries makes them refetch with no token (401 + refresh).
    queryClient.clear()
  }, [])

  const refreshUser = useCallback(async () => {
    const token = getAccessToken()
    if (!token) return
    try {
      const user = await authApi.getMe(token)
      setState((s) => ({ ...s, user }))
    } catch (err) {
      // A failed refetch must not turn an already-saved change into an error.
      console.warn("Could not refresh user", err)
    }
  }, [])

  const can = useCallback(
    (resource: string, action: string) =>
      state.user?.permissions.some(
        (p) => p.resource === resource && p.action === action
      ) ?? false,
    [state.user]
  )

  const scopeOf = useCallback(
    (resource: string, action: string): Scope | null =>
      (state.user?.permissions.find(
        (p) => p.resource === resource && p.action === action
      )?.scope as Scope | undefined) ?? null,
    [state.user]
  )

  const value = useMemo<AuthContextValue>(
    () => ({
      ...state,
      ready: ready.promise,
      login,
      verifyTwoFactor,
      verifyTwoFactorBackup,
      logout,
      refreshUser,
      can,
      scopeOf,
    }),
    [
      state,
      login,
      verifyTwoFactor,
      verifyTwoFactorBackup,
      logout,
      refreshUser,
      can,
      scopeOf,
      ready.promise,
    ]
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function settledStatus(
  auth: Pick<AuthContextValue, "status" | "ready">
): Promise<SettledStatus> | SettledStatus {
  return auth.status === "loading" ? auth.ready : auth.status
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider")
  return ctx
}

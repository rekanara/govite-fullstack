import type { MeResponse } from "@app/contracts"
import { getMe, refreshAccessToken } from "./auth-api"

export interface Session {
  accessToken: string
  user: MeResponse
}

let inFlight: Promise<Session | null> | null = null

export function refreshSession(): Promise<Session | null> {
  if (!inFlight) {
    inFlight = performRefresh().finally(() => {
      inFlight = null
    })
  }
  return inFlight
}

async function performRefresh(): Promise<Session | null> {
  try {
    const { accessToken } = await refreshAccessToken()
    const user = await getMe(accessToken)
    return { accessToken, user }
  } catch {
    return null
  }
}

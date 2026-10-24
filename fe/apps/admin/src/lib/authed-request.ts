import {
  apiBlob,
  apiRequest,
  apiRequestPage,
  ApiError,
  type ApiRequestOptions,
  type Page,
} from "./api-client"
import { getAccessToken, setAccessToken } from "./token-store"
import { refreshSession } from "./refresh-session"
import { toast } from "sonner"

let onSessionExpired: (() => void) | null = null

export function setOnSessionExpired(handler: (() => void) | null): void {
  onSessionExpired = handler
}

function toastApiError(err: unknown): void {
  if (err instanceof ApiError) {
    if (err.status === 401) return
    const title =
      err.status === 403
        ? "Access Denied"
        : err.status >= 500
          ? "Server Error"
          : "Request Failed"
    toast.error(title, { description: err.messageInd ?? err.message })
  } else if (err instanceof TypeError) {
    toast.error("Network Error", {
      description: "Could not reach the server. Check your connection.",
    })
  }
}

async function withRefresh<T>(request: (token: string | null) => Promise<T>) {
  try {
    return await request(getAccessToken())
  } catch (err) {
    if (!(err instanceof ApiError) || err.status !== 401) {
      toastApiError(err)
      throw err
    }
    const session = await refreshSession()
    if (!session) {
      onSessionExpired?.()
      throw err
    }
    setAccessToken(session.accessToken)
    return request(session.accessToken)
  }
}

export function authedRequest<TResponse = void, TBody = undefined>(
  path: string,
  options: ApiRequestOptions<TResponse, TBody> = {}
): Promise<TResponse> {
  return withRefresh((accessToken) =>
    apiRequest(path, { ...options, accessToken })
  )
}

export function authedPage<TItem>(path: string): Promise<Page<TItem>> {
  return withRefresh((accessToken) =>
    apiRequestPage<TItem>(path, { accessToken })
  )
}

export function authedDownload(
  path: string
): Promise<{ blob: Blob; filename: string | null }> {
  return withRefresh((accessToken) => apiBlob(path, accessToken))
}

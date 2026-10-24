const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || "/api/v1"

export interface ResponseStatus {
  code: number
  name: string
  type: "SUCCESS" | "ERROR"
  message: string
  messageInd: string
  timeStamp: string
}

export interface ApiEnvelope<T> {
  data: T | null
  meta?: PageMeta
  error?: { code?: string; detail?: string }
  status: ResponseStatus
}

export interface PageMeta {
  total: number
  page: number
  limit: number
  totalPages: number
  [key: string]: unknown
}

export interface Page<T> {
  data: T[]
  meta: PageMeta
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string | undefined
  readonly messageInd: string | undefined

  constructor(
    status: number,
    code: string | undefined,
    message: string,
    /** Indonesian copy from the envelope, ready to show to users. */
    messageInd?: string
  ) {
    super(message)
    this.status = status
    this.code = code
    this.messageInd = messageInd
  }
}

export interface ApiRequestOptions<TResponse, TBody> {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE"
  body?: TBody
  accessToken?: string | null
  schema?: { parse: (data: unknown) => TResponse }
}

async function send(
  path: string,
  options: ApiRequestOptions<unknown, unknown>
): Promise<{ res: Response; body: ApiEnvelope<unknown> | undefined }> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    method: options.method ?? "GET",
    credentials: "include",
    headers: {
      // FormData must not get a Content-Type: the browser adds the multipart boundary.
      ...(options.body && !(options.body instanceof FormData)
        ? { "Content-Type": "application/json" }
        : {}),
      ...(options.accessToken
        ? { Authorization: `Bearer ${options.accessToken}` }
        : {}),
    },
    body: !options.body
      ? undefined
      : options.body instanceof FormData
        ? options.body
        : JSON.stringify(options.body),
  })

  const text = await res.text()
  let body: ApiEnvelope<unknown> | undefined
  try {
    body = text ? (JSON.parse(text) as ApiEnvelope<unknown>) : undefined
  } catch {
    body = undefined
  }

  if (!res.ok) {
    const detail = body?.error?.detail
    const message = body?.status?.message ?? `Request to ${path} failed`
    throw new ApiError(
      res.status,
      body?.error?.code,
      detail && detail !== message ? `${message}: ${detail}` : message,
      body?.status?.messageInd
    )
  }
  return { res, body }
}

/** Unwraps `data` from the response envelope. */
export async function apiRequest<TResponse = void, TBody = undefined>(
  path: string,
  options: ApiRequestOptions<TResponse, TBody> = {}
): Promise<TResponse> {
  const { body } = await send(path, options)
  const data = body?.data ?? undefined
  return options.schema ? options.schema.parse(data) : (data as TResponse)
}

/** Same, but keeps the list metadata next to the rows. */
export async function apiRequestPage<TItem>(
  path: string,
  options: ApiRequestOptions<TItem[], undefined> = {}
): Promise<Page<TItem>> {
  const { body } = await send(path, options)
  return {
    data: (body?.data ?? []) as TItem[],
    meta: body?.meta ?? {
      total: 0,
      page: 1,
      limit: 0,
      totalPages: 1,
    },
  }
}

// Downloads a binary/text response (e.g. a CSV) instead of parsing JSON.
export async function apiBlob(
  path: string,
  accessToken?: string | null
): Promise<{ blob: Blob; filename: string | null }> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    credentials: "include",
    headers: accessToken ? { Authorization: `Bearer ${accessToken}` } : {},
  })

  if (!res.ok) {
    const text = await res.text()
    let body: ApiEnvelope<unknown> | undefined
    try {
      body = text ? (JSON.parse(text) as ApiEnvelope<unknown>) : undefined
    } catch {
      body = undefined
    }
    throw new ApiError(
      res.status,
      body?.error?.code,
      body?.status?.message ?? `Request to ${path} failed`,
      body?.status?.messageInd
    )
  }

  const disposition = res.headers.get("Content-Disposition") ?? ""
  const filename = /filename="([^"]+)"/.exec(disposition)?.[1] ?? null
  return { blob: await res.blob(), filename }
}

import {
  authedPage,
  authedRequest,
  qs,
} from "@/lib/authed-request"
import type {
  ApiKeyResponse,
  CreateApiKeyRequest,
  CreatedApiKeyResponse,
  UpdateApiKeyRequest,
  WebhookEventQuery,
  WebhookEventResponse,
} from "@app/contracts"

export const webhooksApi = {
  list: (p: WebhookEventQuery) =>
    authedPage<WebhookEventResponse>(`/webhooks/events?${qs({ ...p })}`),
  replay: (id: string) =>
    authedRequest<{ duplicate: boolean }>(`/webhooks/events/${id}/replay`, {
      method: "POST",
    }),
}

export const apiKeysApi = {
  list: () => authedRequest<ApiKeyResponse[]>("/api-keys"),
  create: (body: CreateApiKeyRequest) =>
    authedRequest<CreatedApiKeyResponse, CreateApiKeyRequest>("/api-keys", {
      method: "POST",
      body,
    }),
  update: (id: string, body: UpdateApiKeyRequest) =>
    authedRequest<ApiKeyResponse, UpdateApiKeyRequest>(`/api-keys/${id}`, {
      method: "PATCH",
      body,
    }),
  remove: (id: string) =>
    authedRequest<void>(`/api-keys/${id}`, { method: "DELETE" }),
}

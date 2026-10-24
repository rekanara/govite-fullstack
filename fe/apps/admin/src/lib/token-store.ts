let currentToken: string | null = null

export function getAccessToken(): string | null {
  return currentToken
}

export function setAccessToken(token: string | null): void {
  currentToken = token
}

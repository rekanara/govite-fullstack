/** Saves a fetched blob through a temporary link. */
export function saveDownload(blob: Blob, filename: string | null) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = filename ?? "download"
  a.click()
  URL.revokeObjectURL(url)
}

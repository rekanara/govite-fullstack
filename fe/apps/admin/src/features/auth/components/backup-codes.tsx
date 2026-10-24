import { Button } from "@app/ui/components/button"

export function downloadBackupCodes(codes: string[]) {
  const blob = new Blob(
    [
      `Two-factor backup codes\nEach code works once. Keep them somewhere safe.\n\n${codes.join("\n")}\n`,
    ],
    { type: "text/plain" }
  )
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = "backup-codes.txt"
  a.click()
  URL.revokeObjectURL(url)
}

export function BackupCodes({ codes }: { codes: string[] }) {
  return (
    <div className="space-y-3 rounded-lg border p-3 text-sm">
      <p className="text-muted-foreground">
        Save these backup codes now. Each works once if you lose your
        authenticator. They are shown only this time.
      </p>
      <ul className="grid grid-cols-2 gap-1 font-mono text-xs">
        {codes.map((c) => (
          <li key={c}>{c}</li>
        ))}
      </ul>
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => downloadBackupCodes(codes)}
      >
        Download .txt
      </Button>
    </div>
  )
}

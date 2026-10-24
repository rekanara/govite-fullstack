import { useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import type { UserImportPreviewRow } from "@app/contracts"
import { Button } from "@app/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@app/ui/components/dialog"
import { Input } from "@app/ui/components/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@app/ui/components/table"
import { saveDownload } from "@/components/list-controls"
import { usersApi } from "@/features/users/api"

export function UserImportDialog() {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [rows, setRows] = useState<UserImportPreviewRow[] | null>(null)

  const reset = () => {
    setFile(null)
    setRows(null)
  }
  const template = useMutation({
    mutationFn: usersApi.importTemplate,
    onSuccess: ({ blob, filename }) => saveDownload(blob, filename),
  })
  const preview = useMutation({
    mutationFn: usersApi.previewImport,
    onSuccess: setRows,
  })
  const commit = useMutation({
    mutationFn: usersApi.import,
    onSuccess: async (result) => {
      toast.success(`Invited ${result.created} users`)
      if (result.errors.length > 0)
        toast.warning(`${result.errors.length} rows skipped`)
      await queryClient.invalidateQueries({ queryKey: ["users"] })
      setOpen(false)
      reset()
    },
  })

  const valid = rows?.filter((r) => r.errors.length === 0).length ?? 0
  const invalid = (rows?.length ?? 0) - valid

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) reset()
      }}
    >
      <DialogTrigger asChild>
        <Button variant="outline">Import CSV</Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Import users</DialogTitle>
          <DialogDescription>
            CSV with columns <code>email,fullName,role</code>. Each valid row
            gets an invite email.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-wrap items-center gap-2">
          <Input
            type="file"
            accept=".csv,text/csv"
            className="max-w-xs"
            aria-label="CSV file"
            onChange={(e) => {
              const next = e.currentTarget.files?.[0] ?? null
              setFile(next)
              setRows(null)
              if (next) preview.mutate(next)
            }}
          />
          <Button
            variant="link"
            className="px-0"
            onClick={() => template.mutate()}
          >
            Download template
          </Button>
        </div>

        {preview.isPending && (
          <p className="text-sm text-muted-foreground">Checking file…</p>
        )}

        {rows && (
          <>
            <p role="status" className="text-sm">
              {valid} ready to import
              {invalid > 0 && (
                <span className="text-destructive">
                  {" "}
                  · {invalid} with errors (skipped)
                </span>
              )}
            </p>
            <div className="max-h-72 overflow-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Row</TableHead>
                    <TableHead>Email</TableHead>
                    <TableHead>Name</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Result</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((r) => (
                    <TableRow key={r.row}>
                      <TableCell>{r.row}</TableCell>
                      <TableCell>{r.email || "—"}</TableCell>
                      <TableCell>{r.fullName || "—"}</TableCell>
                      <TableCell>{r.role || "—"}</TableCell>
                      <TableCell
                        className={
                          r.errors.length
                            ? "text-destructive"
                            : "text-emerald-600"
                        }
                      >
                        {r.errors.length ? r.errors.join("; ") : "OK"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          </>
        )}

        <DialogFooter>
          <Button
            disabled={!file || valid === 0 || commit.isPending}
            onClick={() => file && commit.mutate(file)}
          >
            {commit.isPending ? "Importing…" : `Import ${valid} users`}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

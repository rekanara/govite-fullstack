import { useState, type FormEvent } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Badge } from "@app/ui/components/badge"
import { Button } from "@app/ui/components/button"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@app/ui/components/dialog"
import { Input } from "@app/ui/components/input"
import { Label } from "@app/ui/components/label"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@app/ui/components/table"
import { ConfirmAction } from "@/components/confirm-action"
import {
  TableState,
} from "@/components/list-controls"
import { PageHeader } from "@/components/page"
import { RequirePermission } from "@/features/auth/require-permission"
import { apiKeysApi } from "@/features/integrations/api"

export function ApiKeysPage() {
  const [open, setOpen] = useState(false)
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const queryClient = useQueryClient()
  const keys = useQuery({ queryKey: ["api-keys"], queryFn: apiKeysApi.list })
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ["api-keys"] })
  const create = useMutation({
    mutationFn: apiKeysApi.create,
    onSuccess: async (key) => {
      setCreatedKey(key.key)
      toast.success("API key created")
      await refresh()
    },
  })
  const toggle = useMutation({
    mutationFn: ({ id, isActive }: { id: string; isActive: boolean }) =>
      apiKeysApi.update(id, { isActive }),
    onSuccess: refresh,
  })
  const remove = useMutation({
    mutationFn: apiKeysApi.remove,
    onSuccess: refresh,
  })

  function onCreate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    create.mutate({ name: String(form.get("name")) })
  }

  return (
    <RequirePermission resource="apiKeys" action="manage">
      <PageHeader
        title="API keys"
        description="Keys for trusted integrations. Secret is shown once."
        actions={
          <Dialog
            open={open}
            onOpenChange={(v) => {
              setOpen(v)
              if (!v) setCreatedKey(null)
            }}
          >
            <DialogTrigger asChild>
              <Button>Create key</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create API key</DialogTitle>
              </DialogHeader>
              {createdKey ? (
                <div className="grid gap-2">
                  <Label htmlFor="createdKey">Copy this secret now</Label>
                  <Input id="createdKey" value={createdKey} readOnly />
                </div>
              ) : (
                <form id="create-api-key" onSubmit={onCreate}>
                  <div className="grid gap-2">
                    <Label htmlFor="name">Name</Label>
                    <Input id="name" name="name" required />
                  </div>
                </form>
              )}
              <DialogFooter>
                {createdKey ? (
                  <Button onClick={() => setOpen(false)}>Done</Button>
                ) : (
                  <Button
                    type="submit"
                    form="create-api-key"
                    disabled={create.isPending}
                  >
                    Create
                  </Button>
                )}
              </DialogFooter>
            </DialogContent>
          </Dialog>
        }
      />
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Prefix</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Last used</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableState
            colSpan={6}
            loading={keys.isLoading}
            empty={keys.data?.length === 0}
            error={keys.error}
            emptyText="No API keys found."
          />
          {keys.data?.map((key) => (
            <TableRow key={key.id}>
              <TableCell>{key.name}</TableCell>
              <TableCell>{key.prefix}…</TableCell>
              <TableCell>
                <Badge variant={key.isActive ? "default" : "secondary"}>
                  {key.isActive ? "Active" : "Disabled"}
                </Badge>
              </TableCell>
              <TableCell>
                {key.lastUsedAt
                  ? new Date(key.lastUsedAt).toLocaleString()
                  : "—"}
              </TableCell>
              <TableCell>{new Date(key.createdAt).toLocaleString()}</TableCell>
              <TableCell className="space-x-2 text-right">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() =>
                    toggle.mutate({ id: key.id, isActive: !key.isActive })
                  }
                >
                  {key.isActive ? "Disable" : "Enable"}
                </Button>
                <ConfirmAction
                  trigger={
                    <Button size="sm" variant="destructive">
                      Delete
                    </Button>
                  }
                  title="Delete API key?"
                  description="Existing integrations using it will stop working."
                  onConfirm={() => remove.mutate(key.id)}
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </RequirePermission>
  )
}

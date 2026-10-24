import { useState, type FormEvent } from "react"
import { createFileRoute, Link } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Button } from "@app/ui/components/button"
import { Checkbox } from "@app/ui/components/checkbox"
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
import {
  Pagination,
  TableState,
  useDebounced,
} from "@/components/list-controls"
import { PageHeader, RequirePermission } from "@/components/page"
import { rolesApi } from "@/features/roles/api"

export const Route = createFileRoute("/admin/administration/roles")({
  component: RolesPage,
})

function RolesPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(20)
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebounced(search)
  const roles = useQuery({
    queryKey: ["roles", { page, limit, search: debouncedSearch }],
    queryFn: () =>
      rolesApi.list({
        page,
        limit,
        search: debouncedSearch || undefined,
      }),
  })
  const create = useMutation({
    mutationFn: rolesApi.create,
    onSuccess: async () => {
      toast.success("Role created")
      await queryClient.invalidateQueries({ queryKey: ["roles"] })
    },
  })
  const [open, setOpen] = useState(false)

  function onCreate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    create.mutate(
      {
        name: String(form.get("name")),
        description: String(form.get("description") || ""),
        requireTwoFactor: form.get("requireTwoFactor") === "on",
      },
      { onSuccess: () => setOpen(false) }
    )
  }

  const totalPages = roles.data?.meta.totalPages ?? 1

  return (
    <RequirePermission resource="roles" action="manage">
      <PageHeader
        title="Roles"
        description="Role list. Open detail to edit permissions."
        actions={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button>Add role</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>New role</DialogTitle>
              </DialogHeader>
              <form id="create-role" onSubmit={onCreate} className="grid gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="name">Name</Label>
                  <Input id="name" name="name" required />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="description">Description</Label>
                  <Input id="description" name="description" />
                </div>
                <label className="flex items-center gap-2 text-sm">
                  <Checkbox id="requireTwoFactor" name="requireTwoFactor" />
                  Require 2FA
                </label>
              </form>
              <DialogFooter>
                <Button
                  type="submit"
                  form="create-role"
                  disabled={create.isPending}
                >
                  Create
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        }
      />

      <Input
        placeholder="Search role…"
        value={search}
        onChange={(e) => {
          setSearch(e.target.value)
          setPage(1)
        }}
        className="max-w-xs"
      />

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Description</TableHead>
            <TableHead>Users</TableHead>
            <TableHead>Permissions</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>2FA</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableState
            colSpan={7}
            loading={roles.isPending}
            empty={roles.data?.data.length === 0}
            error={roles.error}
            emptyText="No roles found."
          />
          {roles.data?.data.map((role) => (
            <TableRow key={role.id}>
              <TableCell className="font-medium">{role.name}</TableCell>
              <TableCell>{role.description || "—"}</TableCell>
              <TableCell>{role.userCount}</TableCell>
              <TableCell>{role.permissions.length}</TableCell>
              <TableCell>{role.isSystem ? "System" : "Custom"}</TableCell>
              <TableCell>
                {role.requireTwoFactor ? "Required" : "Optional"}
              </TableCell>
              <TableCell className="text-right">
                <Button asChild variant="ghost" size="sm">
                  <Link
                    to="/admin/administration/roles/$roleId"
                    params={{ roleId: role.id }}
                  >
                    View
                  </Link>
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <Pagination
        page={page}
        totalPages={totalPages}
        total={roles.data?.meta.total}
        onPageChange={setPage}
        pageSize={limit}
        onPageSizeChange={(size) => {
          setLimit(size)
          setPage(1)
        }}
      />
    </RequirePermission>
  )
}

import { useState, type FormEvent } from "react"
import { SECURITY_POLICY } from "@app/contracts"
import { createFileRoute, Link } from "@tanstack/react-router"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"
import { Badge } from "@app/ui/components/badge"
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@app/ui/components/select"
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
  saveDownload,
  SortableHead,
  TableState,
  useDebounced,
  type SortState,
} from "@/components/list-controls"
import { ConfirmAction } from "@/components/confirm-action"
import { PageHeader, RequirePermission } from "@/components/page"
import { rolesApi, usersApi, type UserListParams } from "@/features/admin/api"
import { UserImportDialog } from "@/features/admin/user-import-dialog"

export const Route = createFileRoute("/admin/administration/users")({
  component: UsersPage,
})

const ANY = "any"

type UserSort = NonNullable<UserListParams["sort"]>
type BulkAction = "activate" | "deactivate" | "delete" | "restore"

export function UsersPage() {
  const [page, setPage] = useState(1)
  const [search, setSearch] = useState("")
  const [roleId, setRoleId] = useState(ANY)
  const [status, setStatus] = useState(ANY)
  const [deleted, setDeleted] = useState(false)
  const [limit, setLimit] = useState(20)
  const [sort, setSort] = useState<SortState<UserSort>>({
    sort: "createdAt",
    order: "desc",
  })
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const queryClient = useQueryClient()

  const debouncedSearch = useDebounced(search)
  const filters = {
    search: debouncedSearch || undefined,
    roleId: roleId === ANY ? undefined : roleId,
    isActive: status === ANY ? undefined : status === "active",
    deleted,
    ...sort,
  }
  const params = { page, limit, ...filters }
  // Any change to what is listed drops the selection, so bulk actions never
  // hit rows the admin can no longer see.
  const resetPage = () => {
    setPage(1)
    setSelected(new Set())
  }

  const users = useQuery({
    queryKey: ["users", params],
    queryFn: () => usersApi.list(params),
  })
  const roles = useQuery({
    queryKey: ["roles", "options"],
    queryFn: () => rolesApi.list({ page: 1, limit: 100 }),
  })

  const exportUsers = useMutation({
    mutationFn: (format: "csv" | "xlsx") =>
      usersApi.export({ ...filters, format }),
    onSuccess: ({ blob, filename }) => saveDownload(blob, filename),
  })
  const bulk = useMutation({
    mutationFn: (action: BulkAction) =>
      usersApi.bulk({ ids: [...selected], action }),
    onSuccess: async (result) => {
      if (result.succeeded.length)
        toast.success(`${result.succeeded.length} users updated`)
      for (const f of result.failed.slice(0, 3)) toast.error(f.message)
      setSelected(new Set())
      await queryClient.invalidateQueries({ queryKey: ["users"] })
    },
  })

  const create = useMutation({
    mutationFn: usersApi.create,
    onSuccess: async () => {
      toast.success("User created")
      await queryClient.invalidateQueries({ queryKey: ["users"] })
    },
  })

  const [open, setOpen] = useState(false)

  function onCreate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    create.mutate(
      {
        email: String(form.get("email")),
        fullName: String(form.get("fullName")),
        roleId: String(form.get("roleId")),
        // Empty password means "invite by email" on the API side.
        password: String(form.get("password")) || undefined,
      },
      { onSuccess: () => setOpen(false) }
    )
  }

  const totalPages = users.data?.meta.totalPages ?? 1
  const rowIds = users.data?.data.map((u) => u.id) ?? []

  return (
    <RequirePermission resource="users" action="manage">
      <PageHeader
        title="Users"
        description="Accounts and their assigned role."
        actions={
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              onClick={() => exportUsers.mutate("csv")}
              disabled={exportUsers.isPending}
            >
              Export CSV
            </Button>
            <Button
              variant="outline"
              onClick={() => exportUsers.mutate("xlsx")}
              disabled={exportUsers.isPending}
            >
              Export XLSX
            </Button>
            <UserImportDialog />
            <Dialog open={open} onOpenChange={setOpen}>
              <DialogTrigger asChild>
                <Button>Add user</Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>New user</DialogTitle>
                </DialogHeader>
                <form
                  id="create-user"
                  onSubmit={onCreate}
                  className="grid gap-4"
                >
                  <div className="grid gap-2">
                    <Label htmlFor="fullName">Full name</Label>
                    <Input id="fullName" name="fullName" required />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="email">Email</Label>
                    <Input id="email" name="email" type="email" required />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="password">Initial password</Label>
                    <Input
                      id="password"
                      name="password"
                      type="password"
                      minLength={SECURITY_POLICY.passwordMinLength}
                      maxLength={SECURITY_POLICY.passwordMaxLength}
                    />
                    <p className="text-xs text-muted-foreground">
                      Leave empty to email an invite link instead.
                    </p>
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="roleId">Role</Label>
                    <Select name="roleId" required>
                      <SelectTrigger id="roleId">
                        <SelectValue placeholder="Select a role" />
                      </SelectTrigger>
                      <SelectContent>
                        {roles.data?.data.map((role) => (
                          <SelectItem key={role.id} value={role.id}>
                            {role.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </form>
                <DialogFooter>
                  <Button
                    type="submit"
                    form="create-user"
                    disabled={create.isPending}
                  >
                    Create
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>
        }
      />

      <div className="flex flex-wrap items-center gap-2">
        <Input
          placeholder="Search name or email…"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value)
            resetPage()
          }}
          className="max-w-xs"
        />
        <Select
          value={roleId}
          onValueChange={(v) => {
            setRoleId(v)
            resetPage()
          }}
        >
          <SelectTrigger className="w-44">
            <SelectValue placeholder="All roles" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>All roles</SelectItem>
            {roles.data?.data.map((role) => (
              <SelectItem key={role.id} value={role.id}>
                {role.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={status}
          onValueChange={(v) => {
            setStatus(v)
            resetPage()
          }}
        >
          <SelectTrigger className="w-36">
            <SelectValue placeholder="All status" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>All status</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="inactive">Inactive</SelectItem>
          </SelectContent>
        </Select>
        <Button
          variant={deleted ? "default" : "outline"}
          onClick={() => {
            setDeleted((v) => !v)
            resetPage()
          }}
        >
          {deleted ? "Showing deleted" : "Show deleted"}
        </Button>
      </div>

      {selected.size > 0 && (
        <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-muted/40 px-3 py-2 text-sm">
          <span className="font-medium">{selected.size} selected</span>
          {deleted ? (
            <Button
              size="sm"
              variant="outline"
              disabled={bulk.isPending}
              onClick={() => bulk.mutate("restore")}
            >
              Restore
            </Button>
          ) : (
            <>
              <Button
                size="sm"
                variant="outline"
                disabled={bulk.isPending}
                onClick={() => bulk.mutate("activate")}
              >
                Activate
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={bulk.isPending}
                onClick={() => bulk.mutate("deactivate")}
              >
                Deactivate
              </Button>
              <ConfirmAction
                title={`Delete ${selected.size} users?`}
                description="They will be hidden, deactivated and signed out. You can restore them later."
                confirmLabel="Delete"
                onConfirm={() => bulk.mutate("delete")}
                trigger={
                  <Button
                    size="sm"
                    variant="destructive"
                    disabled={bulk.isPending}
                  >
                    Delete
                  </Button>
                }
              />
            </>
          )}
          <Button
            size="sm"
            variant="ghost"
            className="ml-auto"
            onClick={() => setSelected(new Set())}
          >
            Clear
          </Button>
        </div>
      )}

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-10">
              <Checkbox
                aria-label="Select all on this page"
                checked={
                  rowIds.length > 0 && rowIds.every((id) => selected.has(id))
                    ? true
                    : rowIds.some((id) => selected.has(id))
                      ? "indeterminate"
                      : false
                }
                onCheckedChange={(checked) =>
                  setSelected(checked === true ? new Set(rowIds) : new Set())
                }
              />
            </TableHead>
            <SortableHead
              field="fullName"
              label="Name"
              state={sort}
              onChange={(next) => {
                setSort(next)
                resetPage()
              }}
            />
            <SortableHead
              field="email"
              label="Email"
              state={sort}
              onChange={(next) => {
                setSort(next)
                resetPage()
              }}
            />
            <TableHead>Role</TableHead>
            <TableHead>Status</TableHead>
            <SortableHead
              field="lastLoginAt"
              label="Last login"
              state={sort}
              onChange={(next) => {
                setSort(next)
                resetPage()
              }}
            />
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableState
            colSpan={7}
            loading={users.isPending}
            empty={users.data?.data.length === 0}
            error={users.error}
            emptyText="No users found."
          />
          {users.data?.data.map((user) => (
            <TableRow
              key={user.id}
              data-state={selected.has(user.id) ? "selected" : undefined}
            >
              <TableCell>
                <Checkbox
                  aria-label={`Select ${user.email}`}
                  checked={selected.has(user.id)}
                  onCheckedChange={(checked) =>
                    setSelected((prev) => {
                      const next = new Set(prev)
                      if (checked === true) next.add(user.id)
                      else next.delete(user.id)
                      return next
                    })
                  }
                />
              </TableCell>
              <TableCell>{user.fullName ?? "—"}</TableCell>
              <TableCell>{user.email}</TableCell>
              <TableCell>{user.roleName}</TableCell>
              <TableCell>
                <Badge
                  variant={
                    user.deletedAt
                      ? "destructive"
                      : user.invited
                        ? "outline"
                        : user.isActive
                          ? "default"
                          : "secondary"
                  }
                >
                  {user.deletedAt
                    ? "Deleted"
                    : user.invited
                      ? "Invited"
                      : user.isActive
                        ? "Active"
                        : "Inactive"}
                </Badge>
              </TableCell>
              <TableCell>
                {user.lastLoginAt
                  ? new Date(user.lastLoginAt).toLocaleString()
                  : "—"}
              </TableCell>
              <TableCell className="text-right">
                <Button asChild variant="ghost" size="sm">
                  <Link
                    to="/admin/administration/users/$userId"
                    params={{ userId: user.id }}
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
        total={users.data?.meta.total}
        onPageChange={(next) => {
          setPage(next)
          setSelected(new Set())
        }}
        pageSize={limit}
        onPageSizeChange={(size) => {
          setLimit(size)
          resetPage()
        }}
      />
    </RequirePermission>
  )
}

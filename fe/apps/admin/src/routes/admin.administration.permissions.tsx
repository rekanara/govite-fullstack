import { createFileRoute } from "@tanstack/react-router"
import { useQuery } from "@tanstack/react-query"
import { Badge } from "@app/ui/components/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@app/ui/components/table"
import { PageHeader, RequirePermission } from "@/components/page"
import { permissionsApi } from "@/features/admin/api"

export const Route = createFileRoute("/admin/administration/permissions")({
  component: PermissionsPage,
})

function PermissionsPage() {
  const { data, isPending } = useQuery({
    queryKey: ["permissions"],
    queryFn: permissionsApi.catalog,
  })
  return (
    <RequirePermission resource="permissions" action="view">
      <PageHeader
        title="Permissions"
        description="Catalog defined in code (packages/contracts); read-only."
      />
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Resource</TableHead>
            <TableHead>Action</TableHead>
            <TableHead>Key</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {isPending && (
            <TableRow>
              <TableCell colSpan={3}>Loading…</TableCell>
            </TableRow>
          )}
          {data?.permissions.map((p) => (
            <TableRow key={`${p.resource}.${p.action}`}>
              <TableCell>{p.resource}</TableCell>
              <TableCell>{p.action}</TableCell>
              <TableCell>
                <Badge variant="secondary" className="font-mono">
                  {p.resource}.{p.action}
                </Badge>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </RequirePermission>
  )
}

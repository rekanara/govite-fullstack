import { useEffect, useState } from "react"
import { ArrowDownIcon, ArrowUpDownIcon, ArrowUpIcon } from "lucide-react"
import { Button } from "@app/ui/components/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@app/ui/components/select"
import { Skeleton } from "@app/ui/components/skeleton"
import { TableCell, TableHead, TableRow } from "@app/ui/components/table"

export const PAGE_SIZES = [10, 20, 50, 100] as const

export function saveDownload(blob: Blob, filename: string | null) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = filename ?? "download"
  a.click()
  URL.revokeObjectURL(url)
}

export interface SortState<K extends string> {
  sort: K
  order: "asc" | "desc"
}

/** Header cell that toggles asc/desc; aria-sort tells screen readers the state. */
export function SortableHead<K extends string>({
  field,
  label,
  state,
  onChange,
}: {
  field: K
  label: string
  state: SortState<K>
  onChange: (next: SortState<K>) => void
}) {
  const active = state.sort === field
  const Icon = !active
    ? ArrowUpDownIcon
    : state.order === "asc"
      ? ArrowUpIcon
      : ArrowDownIcon
  return (
    <TableHead
      aria-sort={
        active ? (state.order === "asc" ? "ascending" : "descending") : "none"
      }
    >
      <Button
        variant="ghost"
        size="sm"
        className="-ml-2 h-8"
        onClick={() =>
          onChange({
            sort: field,
            order: active && state.order === "asc" ? "desc" : "asc",
          })
        }
      >
        {label}
        <Icon className="size-3.5 text-muted-foreground" />
      </Button>
    </TableHead>
  )
}

export function useDebounced<T>(value: T, ms = 300): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return debounced
}

export function ListState({
  loading,
  empty,
  error,
  emptyText = "No data found.",
}: {
  loading: boolean
  empty: boolean
  error?: Error | null
  emptyText?: string
}) {
  if (error)
    return (
      <p role="alert" className="text-sm text-destructive">
        {error.message}
      </p>
    )
  if (loading) return <Skeleton className="h-16 w-full" />
  if (empty) return <p className="text-sm text-muted-foreground">{emptyText}</p>
  return null
}

export function TableState({
  colSpan,
  loading,
  empty,
  error,
  emptyText = "No data found.",
}: {
  colSpan: number
  loading: boolean
  empty: boolean
  error?: Error | null
  emptyText?: string
}) {
  if (error)
    return (
      <TableRow>
        <TableCell colSpan={colSpan} className="text-destructive">
          {error.message}
        </TableCell>
      </TableRow>
    )
  if (loading)
    return Array.from({ length: 5 }, (_, i) => (
      <TableRow key={i}>
        <TableCell colSpan={colSpan}>
          <Skeleton className="h-5 w-full" />
        </TableCell>
      </TableRow>
    ))
  if (empty)
    return (
      <TableRow>
        <TableCell colSpan={colSpan} className="text-muted-foreground">
          {emptyText}
        </TableCell>
      </TableRow>
    )
  return null
}

export function Pagination({
  page,
  totalPages,
  total,
  onPageChange,
  pageSize,
  onPageSizeChange,
}: {
  page: number
  totalPages: number
  total?: number
  onPageChange: (page: number) => void
  pageSize?: number
  onPageSizeChange?: (size: number) => void
}) {
  return (
    <div className="flex flex-wrap items-center justify-end gap-2">
      {pageSize !== undefined && onPageSizeChange && (
        <Select
          value={String(pageSize)}
          onValueChange={(v) => onPageSizeChange(Number(v))}
        >
          <SelectTrigger className="h-8 w-28" aria-label="Rows per page">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PAGE_SIZES.map((n) => (
              <SelectItem key={n} value={String(n)}>
                {n} / page
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
      <span className="text-sm text-muted-foreground">
        {total !== undefined && `${total} total · `}Page {page} of{" "}
        {Math.max(totalPages, 1)}
      </span>
      <Button
        variant="outline"
        size="sm"
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
      >
        Previous
      </Button>
      <Button
        variant="outline"
        size="sm"
        disabled={page >= totalPages}
        onClick={() => onPageChange(page + 1)}
      >
        Next
      </Button>
    </div>
  )
}

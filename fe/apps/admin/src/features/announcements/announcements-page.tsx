import { useState, type FormEvent } from "react"
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
import { Textarea } from "@app/ui/components/textarea"
import type {
  AnnouncementResponse,
  AnnouncementStatus,
  CreateAnnouncement,
} from "@app/contracts"
import { ConfirmAction } from "@/components/confirm-action"
import {
  Pagination,
  TableState,
} from "@/components/list-controls"
import { useDebounced } from "@/lib/use-debounced"
import { PageHeader } from "@/components/page"
import { RequirePermission } from "@/features/auth/require-permission"
import { announcementsApi } from "@/features/announcements/api"

const ANY = "any"

type Draft = Partial<AnnouncementResponse>

function readForm(form: HTMLFormElement): CreateAnnouncement {
  const data = new FormData(form)
  const imageUrl = String(data.get("imageUrl") ?? "").trim()
  return {
    title: String(data.get("title")),
    content: String(data.get("content")),
    imageUrl: imageUrl || null,
    isPinned: data.get("isPinned") === "on",
    status: String(data.get("status")) as AnnouncementStatus,
  }
}

function AnnouncementDialog({
  draft,
  trigger,
}: {
  draft?: Draft
  trigger: React.ReactNode
}) {
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [imageUrl, setImageUrl] = useState(draft?.imageUrl ?? "")
  const save = useMutation({
    mutationFn: (body: CreateAnnouncement) =>
      draft?.id
        ? announcementsApi.update(draft.id, body)
        : announcementsApi.create(body),
    onSuccess: async () => {
      toast.success(draft?.id ? "Announcement updated" : "Announcement saved")
      await queryClient.invalidateQueries({ queryKey: ["announcements"] })
      setOpen(false)
    },
  })
  const upload = useMutation({
    mutationFn: announcementsApi.uploadImage,
    onSuccess: ({ url }) => {
      setImageUrl(url)
      toast.success("Image uploaded")
    },
  })

  function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    save.mutate(readForm(e.currentTarget))
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {draft?.id ? "Edit announcement" : "New announcement"}
          </DialogTitle>
        </DialogHeader>
        <form id="announcement-form" onSubmit={onSubmit} className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor="title">Title</Label>
            <Input
              id="title"
              name="title"
              defaultValue={draft?.title ?? ""}
              maxLength={200}
              required
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="content">Content</Label>
            <Textarea
              id="content"
              name="content"
              defaultValue={draft?.content ?? ""}
              rows={8}
              maxLength={10000}
              required
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="imageUrl">Image URL</Label>
            <div className="flex gap-2">
              <Input
                id="imageUrl"
                name="imageUrl"
                value={imageUrl}
                onChange={(e) => setImageUrl(e.target.value)}
                placeholder="https://..."
              />
              <Button asChild variant="outline" disabled={upload.isPending}>
                <label>
                  Upload
                  <input
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    className="sr-only"
                    onChange={(e) => {
                      const file = e.currentTarget.files?.[0]
                      if (file) upload.mutate(file)
                    }}
                  />
                </label>
              </Button>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-4">
            <div className="grid gap-2">
              <Label>Status</Label>
              <Select name="status" defaultValue={draft?.status ?? "published"}>
                <SelectTrigger className="w-40">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="draft">Draft</SelectItem>
                  <SelectItem value="published">Published</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <label className="mt-6 flex items-center gap-2 text-sm">
              <Checkbox name="isPinned" defaultChecked={draft?.isPinned} />
              Pin to top
            </label>
          </div>
        </form>
        <DialogFooter>
          <Button
            type="submit"
            form="announcement-form"
            disabled={save.isPending}
          >
            {save.isPending ? "Saving…" : "Save announcement"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export function AnnouncementsPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const [limit, setLimit] = useState(20)
  const [search, setSearch] = useState("")
  const [status, setStatus] = useState(ANY)
  const debouncedSearch = useDebounced(search)
  const params = {
    page,
    limit,
    search: debouncedSearch || undefined,
    status: status === ANY ? undefined : (status as AnnouncementStatus),
  }
  const announcements = useQuery({
    queryKey: ["announcements", params],
    queryFn: () => announcementsApi.list(params),
  })
  const remove = useMutation({
    mutationFn: announcementsApi.remove,
    onSuccess: async () => {
      toast.success("Announcement deleted")
      await queryClient.invalidateQueries({ queryKey: ["announcements"] })
    },
  })

  return (
    <RequirePermission resource="announcements" action="manage">
      <PageHeader
        title="Announcements"
        description="Publish updates to the admin app and mobile devices."
        actions={
          <AnnouncementDialog trigger={<Button>New announcement</Button>} />
        }
      />

      <div className="flex flex-wrap items-center gap-2">
        <Input
          placeholder="Search title or content…"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value)
            setPage(1)
          }}
          className="max-w-xs"
        />
        <Select
          value={status}
          onValueChange={(v) => {
            setStatus(v)
            setPage(1)
          }}
        >
          <SelectTrigger className="w-44">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>All status</SelectItem>
            <SelectItem value="published">Published</SelectItem>
            <SelectItem value="draft">Draft</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Title</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Author</TableHead>
            <TableHead>Published</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableState
            colSpan={5}
            loading={announcements.isPending}
            empty={announcements.data?.data.length === 0}
            error={announcements.error}
            emptyText="No announcements yet."
          />
          {announcements.data?.data.map((item) => (
            <TableRow key={item.id}>
              <TableCell>
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">{item.title}</span>
                    {item.isPinned && <Badge variant="outline">Pinned</Badge>}
                  </div>
                  <p className="line-clamp-2 max-w-xl text-sm text-muted-foreground">
                    {item.content}
                  </p>
                </div>
              </TableCell>
              <TableCell>
                <Badge
                  variant={
                    item.status === "published" ? "default" : "secondary"
                  }
                >
                  {item.status}
                </Badge>
              </TableCell>
              <TableCell>{item.authorName}</TableCell>
              <TableCell>
                {item.publishedAt
                  ? new Date(item.publishedAt).toLocaleString()
                  : "—"}
              </TableCell>
              <TableCell className="space-x-1 text-right">
                <AnnouncementDialog
                  draft={item}
                  trigger={
                    <Button variant="ghost" size="sm">
                      Edit
                    </Button>
                  }
                />
                <ConfirmAction
                  title={`Delete ${item.title}?`}
                  description="This removes the announcement and its uploaded image if unused."
                  confirmLabel="Delete"
                  onConfirm={() => remove.mutate(item.id)}
                  trigger={
                    <Button variant="ghost" size="sm">
                      Delete
                    </Button>
                  }
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>

      <Pagination
        page={page}
        totalPages={announcements.data?.meta.totalPages ?? 1}
        total={announcements.data?.meta.total}
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

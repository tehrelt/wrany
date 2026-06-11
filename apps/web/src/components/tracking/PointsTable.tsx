import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  type ColumnDef,
} from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import { format } from 'date-fns'
import { MapPin, Trash2 } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type { TrackingPoint } from '@/features/tracking/trackingApi'

const MAX_ROWS = 500

interface Props {
  points: TrackingPoint[]
  loading?: boolean
  selectedId?: string | null
  onSelect?: (id: string) => void
  onDelete?: (id: string) => Promise<void>
}

export function PointsTable({ points, loading, selectedId, onSelect, onDelete }: Props) {
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const rows = useMemo(() => points.slice(0, MAX_ROWS), [points])

  const handleDelete = async (eventId: string) => {
    if (!onDelete) return
    setDeletingId(eventId)
    try {
      await onDelete(eventId)
    } finally {
      setDeletingId(null)
    }
  }

  const columns = useMemo<ColumnDef<TrackingPoint>[]>(() => [
    {
      accessorKey: 'recorded_at',
      header: 'Time',
      cell: ({ getValue }) => (
        <span className="font-mono text-xs">
          {format(new Date(getValue<string>()), 'HH:mm:ss.SSS')}
        </span>
      ),
    },
    {
      accessorKey: 'lat',
      header: 'Lat',
      cell: ({ getValue }) => <span className="font-mono text-xs">{getValue<number>().toFixed(6)}</span>,
    },
    {
      accessorKey: 'lon',
      header: 'Lon',
      cell: ({ getValue }) => <span className="font-mono text-xs">{getValue<number>().toFixed(6)}</span>,
    },
    {
      accessorKey: 'accuracy_m',
      header: 'Acc (m)',
      cell: ({ getValue }) => getValue<number>().toFixed(1),
    },
    {
      accessorKey: 'speed_mps',
      header: 'Speed',
      cell: ({ getValue }) => {
        const v = getValue<number | null>()
        return v != null ? `${v.toFixed(2)}` : '—'
      },
    },
    {
      accessorKey: 'activity_type',
      header: 'Activity',
      cell: ({ getValue }) => {
        const v = getValue<string | null>()
        return v ? <Badge variant="secondary" className="text-xs">{v}</Badge> : '—'
      },
    },
    {
      id: 'actions',
      header: '',
      cell: ({ row }) => {
        const id = row.original.event_id
        return (
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              className="h-7 px-2 text-xs gap-1"
              onClick={() => onSelect?.(id)}
            >
              <MapPin className="w-3 h-3" />
              на карте
            </Button>
            {onDelete && (
              <Button
                variant="ghost"
                size="sm"
                className="h-7 px-2 text-destructive hover:text-destructive hover:bg-destructive/10"
                disabled={deletingId === id}
                onClick={() => handleDelete(id)}
              >
                <Trash2 className="w-3 h-3" />
              </Button>
            )}
          </div>
        )
      },
    },
  // eslint-disable-next-line react-hooks/exhaustive-deps
  ], [onSelect, onDelete, deletingId])

  const table = useReactTable({
    data: rows,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  if (loading) {
    return (
      <div className="p-4 flex flex-col gap-2">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-8 w-full" />
        ))}
      </div>
    )
  }

  return (
    <div className="overflow-auto max-h-64 border-t">
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map(hg => (
            <TableRow key={hg.id}>
              {hg.headers.map(header => (
                <TableHead key={header.id}>
                  {flexRender(header.column.columnDef.header, header.getContext())}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.length === 0 ? (
            <TableRow>
              <TableCell colSpan={columns.length} className="text-center text-muted-foreground py-8">
                No points in range
              </TableCell>
            </TableRow>
          ) : (
            table.getRowModel().rows.map(row => (
              <TableRow
                key={row.id}
                data-state={selectedId === row.original.event_id ? 'selected' : undefined}
              >
                {row.getVisibleCells().map(cell => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
      {points.length > MAX_ROWS && (
        <p className="text-xs text-muted-foreground px-4 py-2">
          Showing first {MAX_ROWS} of {points.length} points
        </p>
      )}
    </div>
  )
}

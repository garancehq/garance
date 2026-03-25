import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

interface DataTableProps {
  columns: string[]
  rows: Record<string, unknown>[]
}

export function DataTable({ columns, rows }: DataTableProps) {
  if (columns.length === 0) {
    return <p className="text-zinc-500 text-sm">No data</p>
  }

  return (
    <div className="rounded-md border border-zinc-800 overflow-auto max-h-[600px]">
      <Table>
        <TableHeader>
          <TableRow className="border-zinc-800 hover:bg-transparent">
            {columns.map((col) => (
              <TableHead key={col} className="text-zinc-400 font-mono text-xs">
                {col}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, i) => (
            <TableRow key={i} className="border-zinc-800">
              {columns.map((col) => (
                <TableCell key={col} className="text-zinc-300 font-mono text-xs py-2">
                  {formatValue(row[col])}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return 'NULL'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

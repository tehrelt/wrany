import { format } from 'date-fns'
import { CalendarDays, Clock3 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

interface DateTimePickerProps {
  value: string
  onChange: (iso: string) => void
  label: string
}

export function DateTimePicker({ value, onChange, label }: DateTimePickerProps) {
  const date = value ? new Date(value) : undefined
  const time = date ? format(date, 'HH:mm') : '00:00'

  const updateDate = (next?: Date) => {
    if (!next) return
    const result = new Date(next)
    if (date) result.setHours(date.getHours(), date.getMinutes(), 0, 0)
    onChange(result.toISOString())
  }

  const updateTime = (next: string) => {
    const [hours, minutes] = next.split(':').map(Number)
    const result = date ? new Date(date) : new Date()
    result.setHours(hours, minutes, 0, 0)
    onChange(result.toISOString())
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="outline" className="h-11 w-full justify-start gap-3 bg-card px-3 text-left font-normal hover:border-primary hover:bg-primary/5">
          <CalendarDays className="size-4 text-primary" />
          <span className="min-w-0">
            <span className="block font-mono text-[8px] font-bold uppercase tracking-[0.12em] text-muted-foreground">{label}</span>
            <span className="mono-data block truncate font-mono text-[10px] font-bold">{date ? format(date, 'dd MMM yyyy / HH:mm') : 'Select date'}</span>
          </span>
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="w-auto gap-0 rounded-none border bg-popover p-0 shadow-xl">
        <div className="border-b px-3 py-2 font-mono text-[8px] font-bold uppercase tracking-[0.16em] text-muted-foreground">
          Timing window selector
        </div>
        <Calendar mode="single" selected={date} onSelect={updateDate} />
        <div className="flex items-center gap-3 border-t p-3">
          <Clock3 className="size-4 text-primary" />
          <Input type="time" value={time} onChange={event => updateTime(event.target.value)} aria-label={`${label} time`} className="mono-data h-9 font-mono text-xs" />
        </div>
      </PopoverContent>
    </Popover>
  )
}

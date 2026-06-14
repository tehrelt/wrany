import * as React from 'react'
import { format } from 'date-fns'
import { CalendarIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

interface DateTimePickerProps {
  value: string
  onChange: (iso: string) => void
  label: string
}

export function DateTimePicker({ value, onChange, label }: DateTimePickerProps) {
  const [open, setOpen] = React.useState(false)
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
    <div className="flex flex-col gap-1">
      <Label className="px-1 font-mono text-[9px] font-bold uppercase tracking-[0.14em] text-muted-foreground">
        {label}
      </Label>
      <div className="flex gap-2">
        <Popover open={open} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <Button
              variant="outline"
              data-empty={!date}
              className="h-9 flex-1 justify-start gap-2 px-3 text-left font-normal data-[empty=true]:text-muted-foreground"
            >
              <CalendarIcon className="size-4" />
              <span className="truncate font-mono text-xs">
                {date ? format(date, 'dd MMM yyyy') : 'Select date'}
              </span>
            </Button>
          </PopoverTrigger>
          <PopoverContent className="w-auto overflow-hidden p-0" align="start">
            <Calendar
              mode="single"
              selected={date}
              defaultMonth={date}
              captionLayout="dropdown"
              onSelect={selected => {
                updateDate(selected)
                setOpen(false)
              }}
            />
          </PopoverContent>
        </Popover>
        <Input
          type="time"
          value={time}
          onChange={event => updateTime(event.target.value)}
          aria-label={`${label} time`}
          className="h-9 w-[7.5rem] font-mono text-xs"
        />
      </div>
    </div>
  )
}

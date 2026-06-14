import * as React from 'react'
import { format } from 'date-fns'
import { CalendarIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface DateTimePickerProps {
  value: string
  onChange: (iso: string) => void
  label: string
}

const HOURS = Array.from({ length: 24 }, (_, i) => String(i).padStart(2, '0'))
const MINUTES = Array.from({ length: 12 }, (_, i) => String(i * 5).padStart(2, '0'))

export function DateTimePicker({ value, onChange, label }: DateTimePickerProps) {
  const [open, setOpen] = React.useState(false)
  const date = value ? new Date(value) : undefined
  const hour = date ? String(date.getHours()).padStart(2, '0') : '00'
  // Snap stored minutes to the nearest 5-minute slot for the dropdown value.
  const minute = date ? String(Math.round(date.getMinutes() / 5) * 5 % 60).padStart(2, '0') : '00'

  const updateDate = (next?: Date) => {
    if (!next) return
    const result = new Date(next)
    if (date) result.setHours(date.getHours(), date.getMinutes(), 0, 0)
    onChange(result.toISOString())
  }

  const updateTime = (nextHour: string, nextMinute: string) => {
    const result = date ? new Date(date) : new Date()
    result.setHours(Number(nextHour), Number(nextMinute), 0, 0)
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
        <div className="flex items-center gap-1">
          <Select value={hour} onValueChange={next => updateTime(next, minute)}>
            <SelectTrigger
              aria-label={`${label} hours`}
              className="h-9 w-[3.75rem] font-mono text-xs"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="min-w-[3.75rem]">
              {HOURS.map(h => (
                <SelectItem key={h} value={h} className="font-mono text-xs">
                  {h}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="font-mono text-xs text-muted-foreground">:</span>
          <Select value={minute} onValueChange={next => updateTime(hour, next)}>
            <SelectTrigger
              aria-label={`${label} minutes`}
              className="h-9 w-[3.75rem] font-mono text-xs"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="min-w-[3.75rem]">
              {MINUTES.map(m => (
                <SelectItem key={m} value={m} className="font-mono text-xs">
                  {m}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>
  )
}

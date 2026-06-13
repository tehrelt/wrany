import { ChevronLeft, ChevronRight } from 'lucide-react'
import { DayPicker, type DayPickerProps } from 'react-day-picker'
import { cn } from '@/lib/utils'

export function Calendar({ className, classNames, showOutsideDays = true, ...props }: DayPickerProps) {
  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      className={cn('p-3', className)}
      classNames={{
        root: 'w-fit',
        months: 'flex flex-col',
        month: 'space-y-3',
        month_caption: 'relative flex h-8 items-center justify-center',
        caption_label: 'font-mono text-[10px] font-bold uppercase tracking-[0.14em]',
        nav: 'absolute inset-x-0 top-0 flex items-center justify-between',
        button_previous: 'grid size-8 cursor-pointer place-items-center border bg-background text-muted-foreground hover:border-primary hover:text-primary',
        button_next: 'grid size-8 cursor-pointer place-items-center border bg-background text-muted-foreground hover:border-primary hover:text-primary',
        month_grid: 'w-full border-collapse',
        weekdays: 'flex',
        weekday: 'w-9 py-2 text-center font-mono text-[8px] font-bold uppercase text-muted-foreground',
        week: 'mt-1 flex w-full',
        day: 'relative size-9 p-0 text-center',
        day_button: 'size-9 cursor-pointer font-mono text-[10px] hover:bg-secondary focus-visible:ring-2 focus-visible:ring-ring',
        selected: '[&>button]:bg-primary [&>button]:font-bold [&>button]:text-primary-foreground',
        today: '[&>button]:border [&>button]:border-primary [&>button]:font-bold [&>button]:text-primary',
        outside: 'text-muted-foreground/35',
        disabled: 'text-muted-foreground/25',
        hidden: 'invisible',
        ...classNames,
      }}
      components={{
        Chevron: ({ orientation }) => orientation === 'left'
          ? <ChevronLeft className="size-3.5" />
          : <ChevronRight className="size-3.5" />,
      }}
      {...props}
    />
  )
}

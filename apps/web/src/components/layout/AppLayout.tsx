import { Activity, Flag, LogOut, Map, Radio, Route, UserRound } from 'lucide-react'
import { Link, useLocation } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface AppLayoutProps {
  userEmail: string
  onLogout: () => void
  sidebar: React.ReactNode
  children: React.ReactNode
}

const navigation = [
  { to: '/', label: 'Telemetry', short: 'TEL', icon: Activity },
  { to: '/trips', label: 'Sessions', short: 'SES', icon: Route },
  { to: '/routes', label: 'Circuits', short: 'CIR', icon: Map },
]

export function AppLayout({ userEmail, onLogout, sidebar, children }: AppLayoutProps) {
  const { pathname } = useLocation()
  const current = navigation.find(item => item.to === pathname) ?? navigation[0]

  return (
    <div className="flex h-dvh min-h-[640px] flex-col overflow-hidden bg-background">
      <a href="#main-content" className="fixed left-3 top-3 z-50 -translate-y-20 bg-primary px-3 py-2 text-xs font-bold text-primary-foreground focus:translate-y-0">
        Skip navigation
      </a>

      <header className="relative z-20 flex h-[72px] shrink-0 items-center border-b bg-white text-slate-950 shadow-[0_1px_0_rgba(15,23,42,0.04)]">
        <Link to="/" className="flex h-full w-[180px] shrink-0 items-center gap-3 border-r border-sidebar-border px-4 sm:w-[220px]">
          <span className="cut-corner grid size-10 place-items-center bg-primary text-primary-foreground">
            <Flag className="size-5" />
          </span>
          <span>
            <strong className="block text-lg font-black italic tracking-[-0.06em]">WR ANY%</strong>
            <small className="block font-mono text-[8px] uppercase tracking-[0.2em] text-sidebar-foreground/45">Route race control</small>
          </span>
        </Link>

        <nav className="hidden h-full items-stretch md:flex" aria-label="Primary navigation">
          {navigation.map(({ to, label, short, icon: Icon }) => {
            const active = pathname === to
            return (
              <Link
                key={to}
                to={to}
                className={cn(
                  'relative flex min-w-32 items-center gap-3 border-r border-sidebar-border px-5 transition-colors',
                  active ? 'bg-sidebar-accent text-sidebar-accent-foreground' : 'text-sidebar-foreground/55 hover:bg-sidebar-accent/60 hover:text-sidebar-foreground',
                )}
              >
                {active ? <span className="absolute inset-x-0 bottom-0 h-0.5 bg-primary" /> : null}
                <Icon className={cn('size-4', active && 'text-primary')} />
                <span>
                  <span className="block text-xs font-bold uppercase">{label}</span>
                  <span className="font-mono text-[8px] tracking-[0.18em] opacity-45">{short}</span>
                </span>
              </Link>
            )
          })}
        </nav>

        <div className="ml-auto flex h-full items-center">
          <div className="hidden h-full items-center gap-3 border-l border-sidebar-border px-5 lg:flex">
            <Radio className="size-4 text-primary" />
            <div>
              <p className="font-mono text-[9px] font-bold uppercase tracking-[0.14em]">System live</p>
              <p className="font-mono text-[8px] text-sidebar-foreground/40">AUTO CAPTURE ACTIVE</p>
            </div>
            <span className="signal-light size-2 rounded-full bg-primary text-primary" />
          </div>
          <div className="hidden h-full items-center gap-3 border-l border-sidebar-border px-4 sm:flex">
            <UserRound className="size-4 text-sidebar-foreground/50" />
            <span className="max-w-36 truncate text-xs text-sidebar-foreground/65">{userEmail}</span>
          </div>
          <Button variant="ghost" size="icon" onClick={onLogout} aria-label="Sign out" className="h-full w-16 border-l border-sidebar-border text-sidebar-foreground/55 hover:bg-destructive/10 hover:text-destructive">
            <LogOut className="size-4" />
          </Button>
        </div>
      </header>

      <nav className="flex h-12 shrink-0 border-b bg-sidebar md:hidden" aria-label="Mobile navigation">
        {navigation.map(({ to, label, icon: Icon }) => (
          <Link key={to} to={to} className={cn(
            'flex flex-1 items-center justify-center gap-2 border-r border-sidebar-border text-[10px] font-bold uppercase',
            pathname === to ? 'bg-primary text-primary-foreground' : 'text-sidebar-foreground/55',
          )}>
            <Icon className="size-3.5" />{label}
          </Link>
        ))}
      </nav>

      <div className="flex min-h-0 flex-1">
        <aside className="hidden w-[300px] shrink-0 overflow-y-auto border-r bg-[#f8fafb] p-4 text-slate-950 lg:block">
          <div className="mb-4 flex items-center justify-between border-b pb-3">
            <span className="font-mono text-[9px] font-bold uppercase tracking-[0.18em] text-muted-foreground">Strategy controls</span>
            <span className="font-mono text-[9px] text-primary">{current.short}-01</span>
          </div>
          {sidebar}
        </aside>
        <main id="main-content" className="min-w-0 flex-1 overflow-hidden">
          {children}
        </main>
      </div>
    </div>
  )
}

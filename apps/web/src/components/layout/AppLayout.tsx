import { LogOut, MapPin, Route, Activity } from 'lucide-react'
import { Link, useLocation } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'

interface AppLayoutProps {
  userEmail: string
  onLogout: () => void
  sidebar: React.ReactNode
  children: React.ReactNode
}

export function AppLayout({ userEmail, onLogout, sidebar, children }: AppLayoutProps) {
  const { pathname } = useLocation()

  return (
    <div className="flex flex-col h-screen bg-background">
      <header className="flex items-center justify-between px-4 h-14 border-b shrink-0">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <MapPin className="h-5 w-5 text-primary" />
            <span className="font-semibold text-sm">WR any%</span>
          </div>
          <nav className="flex items-center gap-1">
            <Link to="/">
              <Button
                variant={pathname === '/' ? 'secondary' : 'ghost'}
                size="sm"
                className="gap-1.5 h-8"
              >
                <Activity className="h-4 w-4" />
                <span className="hidden sm:inline">Points</span>
              </Button>
            </Link>
            <Link to="/trips">
              <Button
                variant={pathname === '/trips' ? 'secondary' : 'ghost'}
                size="sm"
                className="gap-1.5 h-8"
              >
                <Route className="h-4 w-4" />
                <span className="hidden sm:inline">Trips</span>
              </Button>
            </Link>
          </nav>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-sm text-muted-foreground hidden sm:block">{userEmail}</span>
          <Button variant="ghost" size="sm" onClick={onLogout} className="gap-1.5">
            <LogOut className="h-4 w-4" />
            <span className="hidden sm:inline">Logout</span>
          </Button>
        </div>
      </header>

      <div className="flex flex-1 overflow-hidden">
        <aside className="w-60 border-r p-4 flex flex-col gap-4 overflow-y-auto shrink-0">
          {sidebar}
        </aside>
        <Separator orientation="vertical" />
        <main className="flex-1 flex flex-col overflow-hidden">
          {children}
        </main>
      </div>
    </div>
  )
}

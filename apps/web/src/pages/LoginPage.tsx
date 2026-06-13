import { useState, type FormEvent } from 'react'
import { Activity, ArrowRight, Flag, Gauge, Route } from 'lucide-react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuth } from '@/features/auth/useAuth'

const telemetry = [
  { icon: Activity, value: 'AUTO', label: 'Movement capture' },
  { icon: Gauge, value: 'LIVE', label: 'Performance data' },
  { icon: Route, value: 'PB', label: 'Route records' },
]

export function LoginPage() {
  const { login, register, loading, error } = useAuth()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault()
    if (mode === 'login') {
      await login({ email, password })
      return
    }
    await register({ email, password })
  }

  return (
    <main className="race-grid min-h-dvh bg-background p-3 sm:p-5">
      <div className="mx-auto grid min-h-[calc(100dvh-1.5rem)] max-w-[1400px] overflow-hidden border bg-card shadow-2xl sm:min-h-[calc(100dvh-2.5rem)] lg:grid-cols-[1.25fr_0.75fr]">
        <section className="speed-lines relative hidden overflow-hidden bg-[#142033] p-10 text-white lg:flex lg:flex-col xl:p-14">
          <div className="absolute -right-16 top-0 h-full w-52 skew-x-[-12deg] border-x border-white/5 bg-white/[0.025]" />
          <div className="flex items-center gap-3">
            <span className="cut-corner grid size-11 place-items-center bg-sidebar-primary text-sidebar-primary-foreground"><Flag className="size-5" /></span>
            <div>
              <strong className="block text-xl font-black italic tracking-[-0.06em]">WR ANY%</strong>
              <span className="font-mono text-[8px] uppercase tracking-[0.22em] text-white/40">Personal route race control</span>
            </div>
          </div>

          <div className="relative my-auto max-w-3xl">
            <div className="mb-6 flex items-center gap-3">
              <span className="h-px w-10 bg-sidebar-primary" />
              <p className="font-mono text-[10px] font-bold uppercase tracking-[0.22em] text-sidebar-primary">Race everyday movement</p>
            </div>
            <h1 className="text-6xl font-black uppercase italic leading-[0.9] tracking-[-0.07em] xl:text-8xl">
              Best laps.<br /><span className="text-sidebar-primary">Real routes.</span>
            </h1>
            <p className="mt-7 max-w-xl text-base leading-7 text-white/55">
              Automatic movement telemetry, repeated route classification, precise timing, and personal records.
            </p>
            <div className="mt-12 grid grid-cols-3 border-y border-white/10">
              {telemetry.map(({ icon: Icon, value, label }) => (
                <div key={label} className="border-r border-white/10 px-4 py-5 last:border-r-0">
                  <Icon className="mb-4 size-4 text-sidebar-primary" />
                  <strong className="block font-mono text-xl">{value}</strong>
                  <span className="mt-1 block font-mono text-[8px] uppercase tracking-[0.14em] text-white/35">{label}</span>
                </div>
              ))}
            </div>
          </div>
          <p className="font-mono text-[8px] uppercase tracking-[0.16em] text-white/25">Original motorsport-inspired telemetry system</p>
        </section>

        <section className="flex items-center justify-center bg-card p-6 sm:p-10 xl:p-16">
          <div className="w-full max-w-sm">
            <div className="mb-12 flex items-center gap-3 lg:hidden">
              <span className="cut-corner grid size-10 place-items-center bg-primary text-primary-foreground"><Flag className="size-4" /></span>
              <strong className="text-lg font-black italic">WR ANY%</strong>
            </div>
            <p className="font-mono text-[10px] font-bold uppercase tracking-[0.2em] text-primary">{mode === 'login' ? 'Driver identification' : 'New driver profile'}</p>
            <h2 className="mt-3 text-4xl font-black uppercase italic tracking-[-0.055em]">{mode === 'login' ? 'Enter race control' : 'Join race control'}</h2>
            <p className="mt-3 text-sm leading-6 text-muted-foreground">{mode === 'login' ? 'Load your routes, sessions, and personal records.' : 'Create your private movement telemetry profile.'}</p>

            <form onSubmit={handleSubmit} className="mt-9 space-y-5">
              <div>
                <Label htmlFor="email" className="font-mono text-[9px] font-bold uppercase tracking-[0.14em]">Driver email</Label>
                <Input id="email" type="email" value={email} onChange={event => setEmail(event.target.value)} required autoComplete="email" placeholder="driver@example.com" className="mt-2 h-12" />
              </div>
              <div>
                <Label htmlFor="password" className="font-mono text-[9px] font-bold uppercase tracking-[0.14em]">Access code</Label>
                <Input id="password" type="password" value={password} onChange={event => setPassword(event.target.value)} required autoComplete={mode === 'login' ? 'current-password' : 'new-password'} className="mt-2 h-12" />
              </div>
              {error ? <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert> : null}
              <Button type="submit" disabled={loading} className="cut-corner h-12 w-full justify-between px-5 font-mono text-[10px] font-bold uppercase tracking-[0.14em]">
                {loading ? 'Connecting...' : mode === 'login' ? 'Open telemetry' : 'Create profile'}
                <ArrowRight className="size-4" />
              </Button>
            </form>

            <button type="button" onClick={() => setMode(mode === 'login' ? 'register' : 'login')} className="mt-7 w-full cursor-pointer text-center text-xs text-muted-foreground hover:text-foreground">
              {mode === 'login' ? 'No profile? Create driver access' : 'Existing profile? Sign in'}
            </button>
          </div>
        </section>
      </div>
    </main>
  )
}

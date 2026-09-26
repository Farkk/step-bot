import { useEffect, useState } from 'react'
import { RealAdmin, type Session } from './RealAdmin'

function App() {
  const [session, setSession] = useState<Session | null>(null)
  const [checking, setChecking] = useState(true)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => { fetch('/api/v1/auth/session', { credentials: 'same-origin' }).then(response => response.ok ? response.json() : null).then(setSession).catch(() => setSession(null)).finally(() => setChecking(false)) }, [])
  async function login(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(''); setBusy(true)
    try {
      const response = await fetch('/api/v1/auth/login', { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email, password }) })
      if (!response.ok) throw new Error(response.status === 401 ? 'Неверная почта или пароль' : 'Не удалось войти. Попробуйте позже.')
      setSession(await response.json())
      setPassword('')
    } catch (e) { setError(e instanceof Error ? e.message : 'Ошибка входа') } finally { setBusy(false) }
  }
  if (checking) return <main className="login-layout"><p className="login-card" role="status">Проверяем вход…</p></main>
  if (session) return <RealAdmin session={session} onLogout={() => setSession(null)} />
  return <main className="login-layout"><section className="login-brand"><div className="login-brand-inner"><img src="/admin/step-logo.png" alt="ШАГ" className="login-logo"/><span className="login-rule"/><p className="login-kicker">КАБИНЕТ ЗАКАЗЧИКА</p><h1>Работа движется<br/><em>шаг за шагом.</em></h1><p className="login-intro">Заявки, отклики и исполнители — в одном месте.</p><div className="login-brand-footer">ШАГ · Управление заявками</div></div></section><section className="login-form-wrap"><div className="login-card"><div className="mobile-logo"><img src="/admin/step-logo.png" alt="ШАГ"/></div><p className="eyebrow">ДОБРО ПОЖАЛОВАТЬ</p><h2>Вход в кабинет</h2><p className="muted">Введите данные корпоративной учётной записи.</p><form onSubmit={login}><label className="field"><span>Email</span><input type="email" value={email} onChange={e => setEmail(e.target.value)} autoComplete="username" required/></label><label className="field"><span>Пароль</span><span className="password-wrap"><input type={showPassword ? 'text' : 'password'} value={password} onChange={e => setPassword(e.target.value)} autoComplete="current-password" required/><button type="button" onClick={() => setShowPassword(!showPassword)} aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}>{showPassword ? 'Скрыть' : 'Показать'}</button></span></label>{error && <p className="form-error" role="alert">{error}</p>}<button className="button button-primary login-submit" type="submit" disabled={busy}>{busy ? 'Входим…' : 'Войти'}</button></form></div></section></main>
}
export default App

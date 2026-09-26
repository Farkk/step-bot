import { useEffect, useState } from 'react'
import { loadProfile, saveProfile, type ContactProof, type SavedProfile } from '../features/profile/api'
import { LOCAL_PREVIEW, resolveLaunchData } from '../features/profile/launch'
import { validateProfile, type ProfileErrors, type ProfileInput } from '../features/profile/profile'
import { GenderSelect } from './GenderSelect'
import { PhoneInput } from './PhoneInput'
import { Dashboard } from './Dashboard'
import './style.css'

const empty: ProfileInput = { fullName: '', phone: '', gender: '', age: '' }

export function App() {
  const [initData, setInitData] = useState('')
  const [profile, setProfile] = useState<SavedProfile | null>(null)
  const [form, setForm] = useState<ProfileInput>(empty)
  const [proof, setProof] = useState<ContactProof | null>(null)
  const [errors, setErrors] = useState<ProfileErrors>({})
  const [status, setStatus] = useState<'loading' | 'form' | 'saved' | 'unavailable'>('loading')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    const data = resolveLaunchData(window.WebApp?.initData || '', window.location.hostname)
    if (!data) { setStatus('unavailable'); return }
    setInitData(data)
    loadProfile(data).then(result => {
      if (result.registered) { setProfile(result.profile); setStatus('saved') }
      else { setForm({ ...empty, fullName: result.suggestedName }); setStatus('form') }
    }).catch(error => { setMessage(error.message); setStatus('unavailable') })
  }, [])

  const localPreview = initData === LOCAL_PREVIEW

  function editLocalProfile() {
    if (!profile) return
    setForm({ fullName: profile.fullName, phone: profile.phone, gender: profile.gender === 'male' || profile.gender === 'female' ? profile.gender : '', age: String(profile.age) })
    setStatus('form')
  }

  function change<K extends keyof ProfileInput>(key: K, value: ProfileInput[K]) {
    setForm(current => ({ ...current, [key]: value }))
    setErrors(current => ({ ...current, [key]: undefined }))
    if (key === 'phone') setProof(null)
  }

  async function takePhoneFromMax() {
    if (!window.WebApp?.requestContact) { setMessage('Запрос телефона недоступен в этой версии MAX. Введите номер вручную.'); return }
    setMessage('')
    try {
      const contact = await window.WebApp.requestContact()
      setForm(current => ({ ...current, phone: contact.phone }))
      setErrors(current => ({ ...current, phone: undefined }))
      setProof(contact)
    } catch { setMessage('MAX не передал номер. Вы можете ввести его вручную.') }
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const nextErrors = validateProfile(form)
    setErrors(nextErrors)
    if (Object.keys(nextErrors).length) return
    setBusy(true)
    setMessage('')
    try {
      const result = await saveProfile(initData, {
        fullName: form.fullName.trim().replace(/\s+/g, ' '), phone: form.phone, gender: form.gender,
        age: Number(form.age), ...(proof ? { contactProof: { authDate: proof.authDate, hash: proof.hash } } : {}),
      })
      if (result.registered) { setProfile(result.profile); setStatus('saved') }
    } catch (error) { setMessage(error instanceof Error ? error.message : 'Не удалось сохранить профиль') }
    finally { setBusy(false) }
  }

  if (status === 'saved' && profile) return <Dashboard profile={profile} localPreview={localPreview} onEditProfile={editLocalProfile} initData={initData} />

  return <main className="page"><section className="shell" aria-labelledby="page-title">
    <header className="hero">
      <img className="logo" src="/app/step-logo.png" alt="Логотип ШАГ" />
      <span className="eyebrow">{localPreview ? 'ЛОКАЛЬНЫЙ ПРОСМОТР' : 'МИНИ-ПРИЛОЖЕНИЕ В MAX'}</span>
      <h1 id="page-title">Первый шаг — ваш профиль</h1>
      <p>Заполните данные один раз, чтобы начать работу в ШАГ.</p>
    </header>
    {status === 'loading' && <p className="notice" role="status">Проверяем данные MAX…</p>}
    {status === 'unavailable' && <div className="notice" role="alert"><strong>Откройте приложение в MAX</strong><p>{message || 'Для сохранения профиля нужны данные запуска MAX.'}</p></div>}
    {status === 'form' && <form className="card form" onSubmit={submit} noValidate>
      <div className="form-heading"><span className="step">01 / 01</span><h2>Личные данные</h2><p>{localPreview ? 'Локальный профиль для проверки формы. Заполните поля вручную.' : 'Имя подставлено из MAX. Проверьте его и заполните оставшиеся поля.'}</p></div>
      <label className="field"><span>ФИО <b>*</b></span><input autoComplete="name" value={form.fullName} onChange={event => change('fullName', event.target.value)} aria-invalid={!!errors.fullName} aria-describedby={errors.fullName ? 'name-error' : undefined} placeholder="Иван Петров" />{errors.fullName && <small id="name-error" className="error">{errors.fullName}</small>}</label>
      <div className="field"><label htmlFor="phone">Телефон <b>*</b></label><div className="phone-row"><PhoneInput value={form.phone} onChange={value => change('phone', value)} invalid={!!errors.phone} describedBy={errors.phone ? 'phone-error' : (localPreview ? undefined : 'phone-help')} />{!localPreview && <button type="button" className="secondary" onClick={takePhoneFromMax}>Из MAX</button>}</div>{!localPreview && <small id="phone-help" className="hint">MAX запросит разрешение перед передачей номера.</small>}{errors.phone && <small id="phone-error" className="error">{errors.phone}</small>}</div>
      <div className="two-fields"><GenderSelect value={form.gender} onChange={value => change('gender', value)} invalid={!!errors.gender} error={errors.gender} /><label className="field"><span>Возраст <b>*</b></span><input type="text" inputMode="numeric" pattern="[0-9]*" autoComplete="off" value={form.age} onChange={event => change('age', event.target.value)} aria-invalid={!!errors.age} aria-describedby={errors.age ? 'age-error' : undefined} placeholder="Лет" />{errors.age && <small id="age-error" className="error">{errors.age}</small>}</label></div>
      {message && <p className="inline-message" role="alert">{message}</p>}
      <button className="primary" type="submit" disabled={busy}>{busy ? 'Сохраняем…' : 'Сохранить профиль'}</button>
    </form>}
    <footer>ШАГ · Двигайтесь к новым задачам</footer>
  </section></main>
}

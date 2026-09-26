import { useState } from 'react'
import { filterExecutors, type Executor } from './executors'

export function ExecutorDirectory({ executors, loading }: { executors: Executor[]; loading: boolean }) {
  const [search, setSearch] = useState('')
  const [minRating, setMinRating] = useState(0)
  const [minCompleted, setMinCompleted] = useState(0)
  const [gender, setGender] = useState('')
  const [minAge, setMinAge] = useState('')
  const [maxAge, setMaxAge] = useState('')
  const [phoneVerified, setPhoneVerified] = useState('')
  const visible = filterExecutors(executors, {
    search, minRating, minCompleted, gender,
    minAge: minAge ? Number(minAge) : undefined,
    maxAge: maxAge ? Number(maxAge) : undefined,
    phoneVerified: phoneVerified === '' ? null : phoneVerified === 'yes',
  })
  const active = !!(search || minRating || minCompleted || gender || minAge || maxAge || phoneVerified)

  function reset() {
    setSearch(''); setMinRating(0); setMinCompleted(0); setGender(''); setMinAge(''); setMaxAge(''); setPhoneVerified('')
  }

  return <>
    <div className="page-heading"><div><span className="eyebrow">БАЗА ИСПОЛНИТЕЛЕЙ</span><h1>Исполнители</h1><p>Все зарегистрированные пользователи бота · {executors.length}</p></div></div>
    <section className="panel executor-directory" aria-label="Список исполнителей">
      <div className="executor-filters">
        <label className="search-field"><span className="sr-only">Поиск исполнителя</span><input value={search} onChange={event => setSearch(event.target.value)} placeholder="Имя или телефон"/></label>
        <label className="filter-select"><span>Рейтинг</span><select value={minRating} onChange={event => setMinRating(Number(event.target.value))}><option value={0}>Любой</option><option value={3}>От 3 ★</option><option value={4}>От 4 ★</option><option value={4.5}>От 4,5 ★</option><option value={5}>5 ★</option></select></label>
        <label className="filter-select"><span>Завершено работ</span><select value={minCompleted} onChange={event => setMinCompleted(Number(event.target.value))}><option value={0}>Любое число</option><option value={1}>От 1</option><option value={5}>От 5</option><option value={10}>От 10</option></select></label>
        <label className="filter-select"><span>Пол</span><select value={gender} onChange={event => setGender(event.target.value)}><option value="">Любой</option><option value="male">Мужской</option><option value="female">Женский</option></select></label>
        <label className="filter-select"><span>Телефон</span><select value={phoneVerified} onChange={event => setPhoneVerified(event.target.value)}><option value="">Любой</option><option value="yes">Подтверждён MAX</option><option value="no">Введён вручную</option></select></label>
        <div className="executor-age-filter"><span>Возраст, лет</span><div><label><span className="sr-only">Возраст от</span><input type="number" min="1" max="120" value={minAge} onChange={event => setMinAge(event.target.value)} placeholder="От"/></label><label><span className="sr-only">Возраст до</span><input type="number" min="1" max="120" value={maxAge} onChange={event => setMaxAge(event.target.value)} placeholder="До"/></label></div></div>
      </div>
      <div className="executor-results"><span>Найдено: {loading ? '…' : visible.length}</span><span>Сначала высокий рейтинг</span>{active && <button type="button" className="text-link" onClick={reset}>Сбросить фильтры</button>}</div>
      {loading ? <p role="status">Загружаем исполнителей…</p> : visible.length ? <div className="executors-list">{visible.map(item => <div className="executor-row" key={item.id}>
        <div><strong>{item.fullName}</strong><small>Зарегистрирован {new Date(item.registeredAt).toLocaleDateString('ru-RU')}</small></div>
        <div><strong>{item.ratings ? `${item.rating.toFixed(1)} ★` : 'Нет оценок'}</strong><small>{item.ratings ? `Оценок: ${item.ratings}` : 'Рейтинг появится после оценки'}</small></div>
        <div><strong>{item.completed}</strong><small>Завершено работ</small></div>
        <div><a href={`tel:${item.phone}`}>{item.phone}</a><small>{item.phoneVerified ? 'Номер подтверждён MAX' : 'Номер введён вручную'}</small></div>
        <span>{item.gender === 'male' ? 'Мужской' : item.gender === 'female' ? 'Женский' : 'Не указан'} · {item.age} лет</span>
      </div>)}</div> : <p className="empty-state">Исполнители по выбранным фильтрам не найдены.</p>}
    </section>
  </>
}

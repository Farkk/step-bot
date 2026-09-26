import { useEffect, useLayoutEffect, useState } from 'react'
import type { SavedProfile } from '../features/profile/api'

type Tab = 'tasks' | 'order' | 'profile'
type IconName = 'list' | 'briefcase' | 'user' | 'bell' | 'arrow' | 'back' | 'pin' | 'clock' | 'wallet' | 'spark' | 'check'

type DemoTask = {
  myApplication?: string
  id: string
  title: string
  category: string
  description: string
  budget: string
  date: string
  place: string
  customer: string
  tags: string[]
  fields?: Record<string, unknown>
  fieldSchema?: {key:string;label:string}[]
  latitude?: number
  longitude?: number
}

type ApiTask = { id: number; company: string; title: string; category: string; description: string; budget: number; deadline: string; location: string; myApplication: string; fields?: Record<string,unknown>; fieldSchema?: {key:string;label:string}[]; latitude?: number; longitude?: number }
type Attachment = { id: number; filename: string }
type Reputation = { acceptedPercent: number; averageRating: number; ratings: number; completed: number }
type Notification = { id: number; taskId: number; title: string; kind: string; createdAt: string }
const notificationLabels: Record<string, string> = { published: 'Новая заявка', application_accepted: 'Ваш отклик принят', application_rejected: 'Ваш отклик отклонён', application_rejected_auto: 'Выбран другой исполнитель', in_progress: 'Заказ в работе', paused: 'Заказ приостановлен', awaiting_confirmation: 'Работа отправлена на подтверждение', completed: 'Заказ завершён', cancelled: 'Заказ отменён', rating_created: 'Получена оценка', rating_updated: 'Оценка изменена' }
function displayTask(task: ApiTask): DemoTask { return { id: String(task.id), title: task.title, category: task.category, description: task.description, budget: task.budget.toLocaleString('ru-RU') + ' ₽', date: new Date(task.deadline).toLocaleString('ru-RU'), place: task.location || 'Место не указано', customer: task.company, tags: [], myApplication: task.myApplication, fields:task.fields, fieldSchema:task.fieldSchema,latitude:task.latitude,longitude:task.longitude } }

function Icon({ name, size = 24 }: { name: IconName; size?: number }) {
  const common = { width: size, height: size, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 1.8, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const, 'aria-hidden': true as const }
  const paths: Record<IconName, React.ReactNode> = {
    list: <><rect x="3" y="4" width="18" height="16" rx="3"/><path d="M8 9h8M8 14h8"/></>,
    briefcase: <><rect x="3" y="7" width="18" height="14" rx="3"/><path d="M8 7V5a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M3 13h18M10 13v2h4v-2"/></>,
    user: <><circle cx="12" cy="8" r="4"/><path d="M4.5 21a7.5 7.5 0 0 1 15 0"/></>,
    bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4"/></>,
    arrow: <path d="m5 12 14 0m-6-6 6 6-6 6"/>,
    back: <path d="m15 5-7 7 7 7"/>,
    pin: <><path d="M19 10c0 5-7 11-7 11S5 15 5 10a7 7 0 1 1 14 0Z"/><circle cx="12" cy="10" r="2"/></>,
    clock: <><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></>,
    wallet: <><rect x="3" y="5" width="18" height="15" rx="3"/><path d="M16 11h5M3 9h18"/></>,
    spark: <path d="m12 2 2.3 6.7L21 11l-6.7 2.3L12 20l-2.3-6.7L3 11l6.7-2.3L12 2Z"/>,
    check: <path d="m5 12 4 4L19 6"/>,
  }
  return <svg {...common}>{paths[name]}</svg>
}

function TaskCard({ task, onOpen }: { task: DemoTask; onOpen: () => void }) {
  return <button className="task-card" type="button" onClick={onOpen} aria-label={`Открыть заявку: ${task.title}`}>
    <span className="task-card-top"><span className="category-label">{task.category}</span><span className="task-card-id">№ {task.id}</span></span>
    <span className="task-title">{task.title}</span>
    <span className="task-meta"><span><Icon name="pin" size={16}/>{task.place}</span><span><Icon name="clock" size={16}/>{task.date}</span></span>
    <span className="task-card-bottom"><span className="task-budget"><small>Вознаграждение</small><strong>{task.budget}</strong></span><span className="task-arrow"><Icon name="arrow" size={20}/></span></span>
  </button>
}

export function Dashboard({ profile, onEditProfile, initData }: { profile: SavedProfile; onEditProfile: () => void; initData: string }) {
  const [tasks, setTasks] = useState<DemoTask[]>([])
  const [orders, setOrders] = useState<(ApiTask & { status: string })[]>([])
  const [taskError, setTaskError] = useState('')
  const [downloadNotice, setDownloadNotice] = useState('')
  const [reputation, setReputation] = useState<Reputation | null>(null)
  const [attachments, setAttachments] = useState<Attachment[]>([])
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [notificationError, setNotificationError] = useState('')
  const [selectedOrderId, setSelectedOrderId] = useState<number | null>(null)
  const [confirmOrderId, setConfirmOrderId] = useState<number | null>(null)
  const [lastSeenNotification, setLastSeenNotification] = useState<number>(() => Number(localStorage.getItem('step-last-notification') || 0))
  const [applying, setApplying] = useState(false)
  const loadTasks = () => fetch('/api/v1/worker/tasks', { headers: { 'X-Max-Init-Data': initData } }).then(async response => { if (!response.ok) throw new Error(response.status===401?'Сессия MAX истекла. Откройте приложение снова.':'Не удалось загрузить заявки'); return response.json() as Promise<ApiTask[]> }).then(items => { const displayed=items.map(displayTask); setTasks(displayed); setSelectedTask(current=>current?displayed.find(item=>item.id===current.id)||null:null); setTaskError('') }).catch(error => setTaskError(error.message))
  const loadOrders = () => fetch('/api/v1/worker/orders', { headers: { 'X-Max-Init-Data': initData } }).then(async response => { if (!response.ok) throw new Error('Не удалось загрузить заказы'); return response.json() as Promise<(ApiTask & { status: string })[]> }).then(setOrders).catch(error => setTaskError(error.message))
  const loadReputation = () => fetch('/api/v1/worker/reputation',{headers:{'X-Max-Init-Data':initData}}).then(async response=>{if(!response.ok)throw new Error('Не удалось загрузить репутацию');return response.json() as Promise<Reputation>}).then(setReputation).catch(error=>setTaskError(error.message))
  const loadNotifications = () => fetch('/api/v1/worker/notifications',{headers:{'X-Max-Init-Data':initData}}).then(async response=>{if(!response.ok)throw new Error('Не удалось загрузить уведомления');return response.json() as Promise<Notification[]>}).then(items=>{setNotifications(items);setNotificationError('')}).catch(error=>setNotificationError(error.message))
  useEffect(() => { const refresh = () => { if (!document.hidden) { void loadTasks(); void loadOrders(); void loadReputation(); void loadNotifications() } }; refresh(); const timer = window.setInterval(refresh, 10000); document.addEventListener('visibilitychange', refresh); window.addEventListener('focus', refresh); return () => { window.clearInterval(timer); document.removeEventListener('visibilitychange', refresh); window.removeEventListener('focus', refresh) } }, [initData])
  async function loadAttachments(id:string){try{const response=await fetch(`/api/v1/worker/tasks/${id}/attachments`,{headers:{'X-Max-Init-Data':initData}});if(!response.ok)throw new Error('Не удалось загрузить файлы');setAttachments(await response.json())}catch(error){setTaskError(error instanceof Error?error.message:'Ошибка файлов')}}
  async function downloadAttachment(file:Attachment){setDownloadNotice('');setTaskError('');try{const response=await fetch(`/api/v1/attachments/${file.id}`,{headers:{'X-Max-Init-Data':initData}});if(!response.ok)throw new Error('Не удалось скачать файл');const blob=await response.blob();const navigatorWithShare=navigator as Navigator & {canShare?:(data:{files:File[]})=>boolean;share?:(data:{files:File[]})=>Promise<void>};const sharedFile=new File([blob],file.filename,{type:blob.type||'application/octet-stream'});if(navigatorWithShare.canShare?.({files:[sharedFile]})){await navigatorWithShare.share?.({files:[sharedFile]});setDownloadNotice(`Файл «${file.filename}» передан в меню сохранения`);return}const url=URL.createObjectURL(blob);const link=document.createElement('a');link.href=url;link.download=file.filename;document.body.appendChild(link);link.click();link.remove();window.setTimeout(()=>URL.revokeObjectURL(url),60000);setDownloadNotice(`Скачивание файла «${file.filename}» началось`)}catch(error){setTaskError(error instanceof Error?error.message:'Ошибка файла')}}
  async function changeOrder(order: ApiTask, action: 'start' | 'complete') { setApplying(true); setTaskError(''); try { const response = await fetch(`/api/v1/worker/orders/${order.id}/status`, { method: 'POST', headers: { 'X-Max-Init-Data': initData, 'Content-Type': 'application/json' }, body: JSON.stringify({ action }) }); if (!response.ok) throw new Error(response.status===409?'Статус заказа уже изменился. Обновите данные.':'Не удалось обновить заказ'); await loadOrders(); await loadNotifications(); setConfirmOrderId(null) } catch (error) { setTaskError(error instanceof Error ? error.message : 'Не удалось обновить заказ') } finally { setApplying(false) } }
  async function apply() { if (!selectedTask || selectedTask.myApplication) return; setApplying(true); setTaskError(''); try { const response = await fetch(`/api/v1/worker/tasks/${selectedTask.id}/applications`, { method: 'POST', headers: { 'X-Max-Init-Data': initData } }); if (!response.ok) throw new Error(response.status === 409 ? 'Вы уже откликнулись или заявка закрыта' : 'Не удалось отправить отклик'); setSelectedTask({ ...selectedTask, myApplication: 'pending' }); await loadTasks() } catch (error) { setTaskError(error instanceof Error ? error.message : 'Не удалось отправить отклик') } finally { setApplying(false) } }
  const [tab, setTab] = useState<Tab>('tasks')
  const [selectedTask, setSelectedTask] = useState<DemoTask | null>(null)
  const [showNotifications, setShowNotifications] = useState(false)
  const [profileList, setProfileList] = useState<'active' | 'archive'>('active')
  const selectedOrder = orders.find(order => order.id === selectedOrderId)
  const unreadCount = notifications.filter(item => item.id > lastSeenNotification).length
  useEffect(()=>{if(showNotifications&&notifications.length&&notifications[0].id>lastSeenNotification){setLastSeenNotification(notifications[0].id);localStorage.setItem('step-last-notification',String(notifications[0].id))}},[showNotifications,notifications,lastSeenNotification])
  const firstName = profile.fullName.trim().split(/\s+/)[0]

  useLayoutEffect(() => { window.scrollTo(0, 0) }, [tab, selectedTask, selectedOrderId, showNotifications])

  function navigate(next: Tab) { setTab(next); setSelectedTask(null); setSelectedOrderId(null); setShowNotifications(false) }
  function openNotifications() { if (!showNotifications) { const latest = notifications[0]?.id || 0; setLastSeenNotification(latest); localStorage.setItem('step-last-notification', String(latest)); void loadNotifications() } setShowNotifications(value => !value); setSelectedTask(null); setSelectedOrderId(null) }
  function openOrder(order: ApiTask) { setSelectedOrderId(order.id); setAttachments([]); void loadAttachments(String(order.id)) }

  return <div className="app-frame">
    <div className="app-content">
      <header className="app-header">
        <div className="brand-lockup"><span className="brand-mark"><img src="/app/step-logo.png" alt="ШАГ"/></span></div>
        <button className="icon-button notification-button" type="button" aria-label={`Уведомления${unreadCount ? `: ${unreadCount} новых` : ''}`} aria-expanded={showNotifications} onClick={openNotifications}><Icon name="bell"/>{unreadCount>0&&<span className="notification-count">{unreadCount}</span>}</button>
      </header>
      {showNotifications ? <main className="screen" id="main-content">
        <button className="back-link" type="button" onClick={() => setShowNotifications(false)}><Icon name="back" size={20}/> Назад</button>
        <div className="screen-heading"><span className="overline">ВАШИ ОБНОВЛЕНИЯ</span><h1>Уведомления</h1><p>Здесь появятся изменения по заявкам и заказам.</p></div>
        {notificationError&&<p className="inline-message" role="alert">{notificationError}</p>}
        {notifications.length?<div className="notification-list">{notifications.map(item=><article className="notification-item" key={item.id}><strong>{notificationLabels[item.kind]||item.kind}</strong><span>{item.title}</span><time>{new Date(item.createdAt).toLocaleString('ru-RU')}</time></article>)}</div>:<div className="empty-panel"><span className="empty-icon"><Icon name="bell" size={28}/></span><h2>Пока нет уведомлений</h2><p>Изменения по заявкам и заказам появятся здесь.</p></div>}
      </main> : selectedOrder ? <main className="screen detail-screen" id="main-content">
        <button className="back-link" type="button" onClick={() => setSelectedOrderId(null)}><Icon name="back" size={20}/> К заказам</button>
        <div className="detail-heading"><span className="category-label">{selectedOrder.category}</span><h1>{selectedOrder.title}</h1><span className="status-pill status-progress">{selectedOrder.status==='awaiting_confirmation'?'Ожидает подтверждения':selectedOrder.status==='assigned'?'Назначен исполнитель':selectedOrder.status==='paused'?'Приостановлена':selectedOrder.status==='completed'?'Завершена':'На исполнении'}</span></div>
        <div className="detail-price"><span>Вознаграждение</span><strong>{selectedOrder.budget.toLocaleString('ru-RU')} ₽</strong></div>
        <section className="detail-section"><h2>О заказе</h2><p>{selectedOrder.description}</p>{Object.entries(selectedOrder.fields||{}).map(([key,value])=><p key={key}>{selectedOrder.fieldSchema?.find(def=>def.key===key)?.label||key}: {String(value)}</p>)}</section>
        <section className="detail-section"><h2>Условия</h2><div className="detail-facts"><div><Icon name="clock" size={20}/><span><small>Начало работ</small>{new Date(selectedOrder.deadline).toLocaleString('ru-RU')}</span></div><div><Icon name="pin" size={20}/><span><small>Место</small>{selectedOrder.location||'Не указано'}</span></div><div><Icon name="user" size={20}/><span><small>Заказчик</small>{selectedOrder.company}</span></div></div></section>
        {attachments.length>0&&<section className="detail-section"><h2>Файлы</h2>{downloadNotice&&<p role="status">{downloadNotice}</p>}{attachments.map(file=><button key={file.id} type="button" className="secondary attachment-download" onClick={()=>downloadAttachment(file)}>Скачать {file.filename}</button>)}</section>}
        {selectedOrder.status==='awaiting_confirmation'&&<p className="inline-message">Вы завершили работу. Заказчик должен подтвердить результат.</p>}
        {taskError&&<p className="inline-message" role="alert">{taskError}</p>}
        {selectedOrder.status==='assigned'&&<button className="primary order-detail-action" disabled={applying} onClick={()=>changeOrder(selectedOrder,'start')}>Взял в работу</button>}
        {selectedOrder.status==='in_progress'&&<button className="primary order-detail-action" disabled={applying} onClick={()=>setConfirmOrderId(selectedOrder.id)}>Завершить</button>}
      </main> : selectedTask ? <main className="screen detail-screen" id="main-content">
        <button className="back-link" type="button" onClick={() => setSelectedTask(null)}><Icon name="back" size={20}/> К заявкам</button>
        <div className="detail-heading"><span className="category-label">{selectedTask.category}</span><h1>{selectedTask.title}</h1><span className="status-pill status-open">Открыта</span></div>
        <div className="detail-price"><span>Вознаграждение</span><strong>{selectedTask.budget}</strong></div>
        <section className="detail-section"><h2>О задаче</h2><p>{selectedTask.description}</p><div className="tag-row">{selectedTask.tags.map(tag => <span key={tag}>{tag}</span>)}</div>{Object.entries(selectedTask.fields||{}).map(([key,value])=><p key={key}>{selectedTask.fieldSchema?.find(def=>def.key===key)?.label||key}: {String(value)}</p>)}</section>
        <section className="detail-section"><h2>Условия</h2><div className="detail-facts"><div><Icon name="clock" size={20}/><span><small>Когда</small>{selectedTask.date}</span></div><div><Icon name="pin" size={20}/><span><small>Где</small>{selectedTask.place}</span></div><div><Icon name="user" size={20}/><span><small>Заказчик</small>{selectedTask.customer}</span></div></div></section>
        {selectedTask.latitude!=null&&<p>Координаты: {selectedTask.latitude}, {selectedTask.longitude}</p>}{attachments.length>0&&<section className="detail-section"><h2>Файлы</h2>{downloadNotice&&<p role="status">{downloadNotice}</p>}{attachments.map(file=><button key={file.id} type="button" className="secondary attachment-download" onClick={()=>downloadAttachment(file)}>Скачать {file.filename}</button>)}</section>}
        <div className="detail-action"><button className="primary" type="button" disabled={applying || !!selectedTask.myApplication} onClick={apply}>{selectedTask.myApplication ? 'Отклик отправлен' : applying ? 'Отправляем…' : 'Откликнуться'}</button>{taskError && <p role="alert">{taskError}</p>}</div>
      </main> : tab === 'tasks' ? <main className="screen" id="main-content">
        <div className="home-heading"><span className="home-greeting">Привет, {firstName}</span><span className="heading-spark" aria-hidden="true">✦</span><h1>Новые <em>заявки</em></h1><p>Выбирайте подходящие задачи.</p></div>
        <div className="feed-heading"><div><h2>Доступны сейчас</h2><span>Задачи рядом с вами</span></div><span className="feed-count">{tasks.length}</span></div>
        {taskError && <p className="inline-message" role="alert">{taskError}</p>}
        <div className="task-list">{tasks.map(task => <TaskCard key={task.id} task={task} onOpen={() => {setSelectedTask(task);void loadAttachments(task.id)}}/>)}</div>{tasks.length === 0 && !taskError && <div className="empty-panel"><h2>Пока нет заявок</h2><p>Новые задания появятся здесь после публикации заказчиком.</p></div>}
      </main> : tab === 'order' ? <main className="screen" id="main-content">
        <div className="screen-heading"><span className="overline">МОЯ РАБОТА</span><h1>Активный заказ</h1><p>Назначенные вам заявки.</p></div>
        {taskError && <p className="inline-message" role="alert">{taskError}</p>}
        {orders.filter(order => !['completed','cancelled'].includes(order.status)).length ? <div className="order-list">{orders.filter(order => !['completed','cancelled'].includes(order.status)).map(order => <section className="order-hero" key={order.id}><span className={`status-pill ${order.status === 'paused' ? 'status-neutral' : order.status === 'awaiting_confirmation' ? 'status-open' : 'status-progress'}`}>{order.status === 'assigned' ? 'Назначен исполнитель' : order.status === 'paused' ? 'Приостановлена' : order.status === 'awaiting_confirmation' ? 'Ожидает подтверждения' : 'На исполнении'}</span><h2>{order.title}</h2><p>{order.description}</p><button className="order-open" type="button" onClick={()=>openOrder(order)}>Подробнее о заказе <Icon name="arrow" size={18}/></button><div className="detail-facts"><div><Icon name="clock" size={20}/><span><small>Начало работ</small>{new Date(order.deadline).toLocaleString('ru-RU')}</span></div><div><Icon name="pin" size={20}/><span><small>Место</small>{order.location || 'Не указано'}</span></div><div><Icon name="wallet" size={20}/><span><small>Бюджет</small>{order.budget.toLocaleString('ru-RU')} ₽</span></div></div>{order.status==='awaiting_confirmation'&&<p className="order-state-hint">Работа отправлена заказчику на подтверждение.</p>}<div className="order-actions">{order.status==='assigned'&&<button className="primary" disabled={applying} onClick={() => changeOrder(order, 'start')}>Взял в работу</button>}{order.status==='in_progress'&&<button className="primary" disabled={applying} onClick={() => setConfirmOrderId(order.id)}>Завершить</button>}</div></section>)}</div> : <div className="empty-panel"><h2>Активного заказа нет</h2><p>Когда заказчик примет ваш отклик, заказ появится здесь.</p></div>}
      </main> : <main className="screen" id="main-content">
        <div className="screen-heading profile-heading"><span className="overline">ВАШ ПРОФИЛЬ</span><h1>Мой профиль</h1></div>
        <section className="profile-summary"><span className="profile-summary-label">Личные данные</span><div className="profile-summary-main"><div><h2>{profile.fullName}</h2><p>Исполнитель</p></div><button className="profile-edit-link" type="button" onClick={onEditProfile}>Изменить</button></div><div className="profile-phone"><span>Телефон</span><strong>{profile.phone}</strong></div></section>
        <section className="reputation"><div className="section-title"><h2>Репутация</h2></div><div className="metric-grid"><div><strong>{reputation?Math.round(reputation.acceptedPercent)+'%':'—'}</strong><span>Принятых откликов</span></div><div><strong>{reputation&&reputation.ratings?reputation.averageRating.toFixed(1):'—'}</strong><span>Средний балл · {reputation?.ratings??0} оценок</span></div></div><p>Доля принятых откликов — назначения среди рассмотренных откликов. Завершено работ: {reputation?.completed??0}.</p></section>
        <section className="profile-tasks"><h2>Мои задачи</h2><div className="segmented" role="group" aria-label="Список задач"><button type="button" aria-pressed={profileList === 'active'} onClick={() => setProfileList('active')}>Активные</button><button type="button" aria-pressed={profileList === 'archive'} onClick={() => setProfileList('archive')}>Архив</button></div>{orders.filter(order => profileList === 'archive' ? ['completed','cancelled'].includes(order.status) : !['completed','cancelled'].includes(order.status)).map(order => <div className="profile-task-card" key={order.id}><span className={`status-pill ${order.status === 'completed' ? 'status-done' : order.status === 'cancelled' || order.status === 'paused' ? 'status-neutral' : order.status === 'awaiting_confirmation' ? 'status-open' : 'status-progress'}`}>{order.status === 'completed' ? 'Завершена' : order.status === 'cancelled' ? 'Отменена' : order.status === 'assigned' ? 'Назначен исполнитель' : order.status === 'paused' ? 'Приостановлена' : order.status === 'awaiting_confirmation' ? 'Ожидает подтверждения' : 'На исполнении'}</span><h3>{order.title}</h3><p>{new Date(order.deadline).toLocaleString('ru-RU')} · {order.location || 'Место не указано'}</p></div>)}{orders.filter(order => profileList === 'archive' ? ['completed','cancelled'].includes(order.status) : !['completed','cancelled'].includes(order.status)).length === 0 && <div className="empty-panel compact"><p>Здесь пока нет задач.</p></div>}</section>
      </main>}
    </div>
    {confirmOrderId!==null&&<div className="confirm-scrim" role="presentation"><div className="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="confirm-title"><h2 id="confirm-title">Завершить работу?</h2><p>Заказ перейдёт в ожидание подтверждения заказчиком.</p>{taskError&&<p role="alert">{taskError}</p>}<div><button className="secondary" type="button" disabled={applying} onClick={()=>setConfirmOrderId(null)}>Отмена</button><button className="primary" type="button" disabled={applying} onClick={()=>{const order=orders.find(item=>item.id===confirmOrderId);if(order)void changeOrder(order,'complete')}}>{applying?'Сохраняем…':'Завершить работу'}</button></div></div></div>}
    <nav className="bottom-nav" aria-label="Главное меню"><div className="bottom-nav-inner">
      <button type="button" className={tab === 'tasks' && !showNotifications ? 'active' : ''} aria-current={tab === 'tasks' && !showNotifications ? 'page' : undefined} onClick={() => navigate('tasks')}><Icon name="list"/><span>Заявки</span></button>
      <button type="button" className={tab === 'order' && !showNotifications ? 'active' : ''} aria-current={tab === 'order' && !showNotifications ? 'page' : undefined} onClick={() => navigate('order')}><Icon name="briefcase"/><span>Активный заказ</span></button>
      <button type="button" className={tab === 'profile' && !showNotifications ? 'active' : ''} aria-current={tab === 'profile' && !showNotifications ? 'page' : undefined} onClick={() => navigate('profile')}><Icon name="user"/><span>Профиль</span></button>
    </div></nav>
  </div>
}

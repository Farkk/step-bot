import { useEffect, useState } from 'react'

type Attachment = { id: number; filename: string; contentType: string }

export function AttachmentGallery({ files, onDownload }: { files: Attachment[]; onDownload: (file: Attachment) => void }) {
  const [urls, setUrls] = useState<Record<number,string>>({})
  const [preview, setPreview] = useState<Attachment | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    const created: string[] = []
    setUrls({})
    setPreview(null)
    setError('')
    void Promise.all(files.filter(file => file.contentType.startsWith('image/')).map(async file => {
      try {
        const response = await fetch(`/api/v1/attachments/${file.id}`, { credentials: 'same-origin' })
        if (!response.ok) throw new Error()
        const url = URL.createObjectURL(await response.blob())
        created.push(url)
        if (!cancelled) setUrls(current => ({ ...current, [file.id]: url }))
      } catch { if (!cancelled) setError('Не удалось загрузить фото') }
    }))
    return () => { cancelled = true; created.forEach(URL.revokeObjectURL) }
  }, [files])

  useEffect(() => {
    if (!preview) return
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') setPreview(null) }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [preview])

  if (!files.length) return <p>Файлов пока нет.</p>
  return <>
    {error && <p role="alert">{error}</p>}
    <div className="admin-attachment-list">{files.map(file => file.contentType.startsWith('image/') ?
      <div className="admin-photo-item" key={file.id}>
        <button type="button" className="admin-photo-preview" onClick={() => setPreview(file)} disabled={!urls[file.id]} aria-label={`Открыть фото ${file.filename}`}>
          {urls[file.id] ? <img src={urls[file.id]} alt={file.filename}/> : <span>Загружаем фото…</span>}
        </button>
        <button type="button" className="attachment-link" onClick={() => onDownload(file)}>Сохранить фото</button>
      </div> : <button key={file.id} type="button" className="attachment-link" onClick={() => onDownload(file)}>Скачать {file.filename}</button>)}</div>
    {preview && urls[preview.id] && <div className="admin-photo-backdrop" role="presentation" onClick={() => setPreview(null)}><div className="admin-photo-dialog" role="dialog" aria-modal="true" aria-label={`Фото ${preview.filename}`} onClick={event => event.stopPropagation()}><div><span>{preview.filename}</span><button type="button" onClick={() => setPreview(null)} aria-label="Закрыть фото">✕</button></div><img src={urls[preview.id]} alt={preview.filename}/><button type="button" className="button button-outline" onClick={() => onDownload(preview)}>Сохранить фото</button></div></div>}
  </>
}

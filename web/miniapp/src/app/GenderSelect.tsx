import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import type { Gender } from '../features/profile/profile'

const choices = [
  { value: 'male', label: 'Мужской' },
  { value: 'female', label: 'Женский' },
] as const

type Props = {
  value: Gender
  onChange: (value: Gender) => void
  invalid: boolean
  error?: string
}

export function GenderSelect({ value, onChange, invalid, error }: Props) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const options = useRef<(HTMLButtonElement | null)[]>([])
  const id = useId()

  useEffect(() => {
    if (!open) return
    options.current[Math.max(0, choices.findIndex(choice => choice.value === value))]?.focus()
    const closeOutside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', closeOutside)
    return () => document.removeEventListener('pointerdown', closeOutside)
  }, [open, value])

  function choose(next: Gender) {
    onChange(next)
    setOpen(false)
    trigger.current?.focus()
  }

  function optionKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let next = index
    if (event.key === 'ArrowDown') next = (index + 1) % choices.length
    else if (event.key === 'ArrowUp') next = (index - 1 + choices.length) % choices.length
    else if (event.key === 'Home') next = 0
    else if (event.key === 'End') next = choices.length - 1
    else if (event.key === 'Escape') {
      event.preventDefault()
      setOpen(false)
      trigger.current?.focus()
      return
    } else if (event.key === 'Tab') {
      setOpen(false)
      return
    } else return
    event.preventDefault()
    options.current[next]?.focus()
  }

  return <div className="field gender-field" ref={root}>
    <span id={`${id}-label`}>Пол <b>*</b></span>
    <div className="select-control"><button ref={trigger} type="button" className="select-trigger" aria-labelledby={`${id}-label ${id}-value`} aria-haspopup="listbox" aria-expanded={open} aria-controls={open ? `${id}-list` : undefined} aria-invalid={invalid} aria-describedby={error ? `${id}-error` : undefined} onClick={() => setOpen(current => !current)} onKeyDown={event => {
      if (!open && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) { event.preventDefault(); setOpen(true) }
      if (open && event.key === 'Escape') { event.preventDefault(); setOpen(false) }
    }}>
      <span id={`${id}-value`} className={value ? '' : 'select-placeholder'}>{choices.find(choice => choice.value === value)?.label || 'Выберите'}</span>
      <svg className="select-chevron" viewBox="0 0 20 20" width="20" height="20" aria-hidden="true"><path d="m5 7.5 5 5 5-5" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>
    </button>
    {open && <div id={`${id}-list`} className="select-list" role="listbox" aria-labelledby={`${id}-label`}>
      {choices.map((choice, index) => <button key={choice.value} ref={element => { options.current[index] = element }} type="button" className="select-option" role="option" aria-selected={value === choice.value} onClick={() => choose(choice.value)} onKeyDown={event => optionKeyDown(event, index)}>{choice.label}</button>)}
    </div>}</div>
    {error && <small id={`${id}-error`} className="error">{error}</small>}
  </div>
}

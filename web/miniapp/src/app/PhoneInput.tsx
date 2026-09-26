import { useRef, type ChangeEvent, type KeyboardEvent } from 'react'
import { formatRussianPhone, phoneCaretPosition, subscriberDigits } from '../features/profile/phone'

type Props = {
  value: string
  onChange: (value: string) => void
  invalid: boolean
  describedBy?: string
}

export function PhoneInput({ value, onChange, invalid, describedBy }: Props) {
  const input = useRef<HTMLInputElement>(null)

  function placeCaret(position: number) {
    requestAnimationFrame(() => input.current?.setSelectionRange(position, position))
  }

  function handleChange(event: ChangeEvent<HTMLInputElement>) {
    const raw = event.target.value
    const count = subscriberDigits(raw.slice(0, event.target.selectionStart ?? raw.length)).length
    const formatted = formatRussianPhone(raw)
    onChange(formatted)
    placeCaret(phoneCaretPosition(formatted, count))
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key !== 'Backspace' && event.key !== 'Delete') return
    const element = event.currentTarget
    const start = element.selectionStart ?? 0
    if (start !== element.selectionEnd) return
    if (event.key === 'Backspace' && start <= 4) { event.preventDefault(); return }
    const adjacent = event.key === 'Backspace' ? start - 1 : start
    if (/\d/.test(value[adjacent] || '')) return
    let digit = adjacent
    const direction = event.key === 'Backspace' ? -1 : 1
    while (digit >= 4 && digit < value.length && !/\d/.test(value[digit])) digit += direction
    if (digit < 4 || digit >= value.length) return
    event.preventDefault()
    const count = subscriberDigits(value.slice(0, digit)).length
    const formatted = formatRussianPhone(value.slice(0, digit) + value.slice(digit + 1))
    onChange(formatted)
    placeCaret(phoneCaretPosition(formatted, count))
  }

  return <input ref={input} id="phone" type="tel" autoComplete="tel" inputMode="tel" value={value} onChange={handleChange} onKeyDown={handleKeyDown} onFocus={() => { if (!value) { onChange('+7 ('); placeCaret(4) } }} aria-invalid={invalid} aria-describedby={describedBy} placeholder="+7 (999) 123-45-67" />
}

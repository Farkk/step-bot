export function subscriberDigits(value: string): string {
  const digits = value.replace(/\D/g, '')
  return (digits.startsWith('7') || digits.startsWith('8') ? digits.slice(1) : digits).slice(0, 10)
}

export function formatRussianPhone(value: string): string {
  if (!value) return ''
  const digits = subscriberDigits(value)
  let formatted = '+7 (' + digits.slice(0, 3)
  if (digits.length >= 3) formatted += ')'
  if (digits.length > 3) formatted += ' ' + digits.slice(3, 6)
  if (digits.length > 6) formatted += '-' + digits.slice(6, 8)
  if (digits.length > 8) formatted += '-' + digits.slice(8, 10)
  return formatted
}

export function phoneCaretPosition(formatted: string, subscriberCount: number): number {
  if (!subscriberCount) return Math.min(4, formatted.length)
  let count = 0
  for (let index = 4; index < formatted.length; index++) {
    if (/\d/.test(formatted[index])) count++
    if (count === subscriberCount) {
      let position = index + 1
      while (position < formatted.length && !/\d/.test(formatted[position])) position++
      return position
    }
  }
  return formatted.length
}

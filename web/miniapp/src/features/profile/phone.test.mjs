import test from 'node:test'
import assert from 'node:assert/strict'
import { formatRussianPhone, phoneCaretPosition, subscriberDigits } from './phone.ts'
import { validateProfile } from './profile.ts'

test('маска форматирует набор и вставку российского номера', () => {
  assert.equal(formatRussianPhone(''), '')
  assert.equal(formatRussianPhone('9'), '+7 (9')
  assert.equal(formatRussianPhone('9991234567'), '+7 (999) 123-45-67')
  assert.equal(formatRussianPhone('89991234567'), '+7 (999) 123-45-67')
  assert.equal(formatRussianPhone('+79991234567'), '+7 (999) 123-45-67')
  assert.equal(formatRussianPhone('8999123456789'), '+7 (999) 123-45-67')
  assert.equal(formatRussianPhone('7'), '+7 (')
})

test('курсор следует за цифрами при вводе внутри маски', () => {
  assert.equal(subscriberDigits('+7 (999) 123'.slice(0, 7)), '999')
  assert.equal(phoneCaretPosition('+7 (999) 123', 3), 9)
})

test('неполный номер с маской не проходит проверку', () => {
  const input = { fullName: 'Иван Петров', phone: '+7 (999) 123-45-6', gender: 'male', age: '25' }
  assert.equal(validateProfile(input).phone, 'Проверьте номер телефона')
})

import test from 'node:test'
import assert from 'node:assert/strict'
import { validateProfile } from './profile.ts'

test('профиль требует ФИО, телефон, пол и допустимый возраст', () => {
  assert.deepEqual(validateProfile({ fullName: '', phone: '', gender: '', age: '' }), {
    fullName: 'Укажите ФИО', phone: 'Укажите телефон', gender: 'Выберите пол', age: 'Укажите возраст',
  })
  assert.equal(validateProfile({ fullName: 'Иван Петров', phone: '+7 999 123-45-67', gender: 'male', age: '25' }).phone, undefined)
  assert.equal(validateProfile({ fullName: 'Иван Петров', phone: '123', gender: 'male', age: '200' }).phone, 'Проверьте номер телефона')
})

test('вариант пола «Другой» больше не принимается', () => {
  assert.equal(validateProfile({ fullName: 'Иван Петров', phone: '+7 999 123-45-67', gender: 'other', age: '25' }).gender, 'Выберите пол')
})

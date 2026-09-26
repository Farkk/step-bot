import assert from 'node:assert/strict'
import test from 'node:test'
import { filterExecutors } from './executors.ts'

const executors = [
  { id: 1, fullName: 'Иван Петров', phone: '+7 111', gender: 'male', age: 28, rating: 0, ratings: 0, completed: 0, phoneVerified: false },
  { id: 2, fullName: 'Мария Соколова', phone: '+7 222', gender: 'female', age: 34, rating: 4.5, ratings: 8, completed: 12, phoneVerified: true },
  { id: 3, fullName: 'Игорь Павлов', phone: '+7 333', gender: 'male', age: 41, rating: 5, ratings: 2, completed: 3, phoneVerified: true },
  { id: 4, fullName: 'Ольга Иванова', phone: '+7 444', gender: 'female', age: 29, rating: 5, ratings: 5, completed: 7, phoneVerified: false },
]

test('по умолчанию рейтинг убывает, при равенстве учитывается число оценок, без оценок внизу', () => {
  assert.deepEqual(filterExecutors(executors, {}).map(item => item.id), [4, 3, 2, 1])
})

test('поиск и фильтры применяются совместно', () => {
  assert.deepEqual(filterExecutors(executors, { minRating: 4.5, minCompleted: 5, gender: 'female' }).map(item => item.id), [4, 2])
  assert.deepEqual(filterExecutors(executors, { search: '333', minRating: 4 }).map(item => item.id), [3])
  assert.deepEqual(filterExecutors(executors, { minAge: 30, maxAge: 40, phoneVerified: true }).map(item => item.id), [2])
})

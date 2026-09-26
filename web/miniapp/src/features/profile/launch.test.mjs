import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveLaunchData } from './launch.ts'

test('MAX данные имеют приоритет', () => {
  assert.equal(resolveLaunchData('signed-data', 'localhost'), 'signed-data')
})

test('локальный просмотр доступен только на loopback', () => {
  assert.equal(resolveLaunchData('', 'localhost'), 'local-preview')
  assert.equal(resolveLaunchData('', '127.0.0.1'), 'local-preview')
  assert.equal(resolveLaunchData('', 'step.example.com'), '')
})

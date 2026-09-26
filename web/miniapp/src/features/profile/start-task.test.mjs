import assert from 'node:assert/strict'
import test from 'node:test'
import { taskIdFromLaunch } from './launch.ts'

test('ссылка MAX открывает только заявку с числовым номером', () => {
  assert.equal(taskIdFromLaunch('start_param=task_42', ''), '42')
  assert.equal(taskIdFromLaunch('', '?WebAppStartParam=task_15'), '15')
  assert.equal(taskIdFromLaunch('start_param=task_x', ''), null)
  assert.equal(taskIdFromLaunch('start_param=task_42%2Fother', ''), null)
})

import { getJSON } from './client'

export function getHealth() {
  return getJSON<{ status: string }>('/health/ready')
}

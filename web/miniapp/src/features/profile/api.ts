import type { Gender } from './profile'

declare global {
  interface Window {
    WebApp?: {
      initData: string
      requestContact?: () => Promise<{ phone: string; authDate: string; hash: string }>
    }
  }
}

export type SavedProfile = { fullName: string; phone: string; gender: Gender | 'other'; age: number; phoneVerified: boolean }
export type ProfileResponse = { registered: false; suggestedName: string } | { registered: true; profile: SavedProfile }
export type ContactProof = { phone: string; authDate: string; hash: string }

async function request(initData: string, method: 'GET' | 'PUT', body?: unknown): Promise<ProfileResponse> {
  const response = await fetch('/api/v1/me/profile', {
    method,
    headers: { 'X-Max-Init-Data': initData, ...(body ? { 'Content-Type': 'application/json' } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  })
  if (!response.ok) throw new Error(response.status === 401 ? 'Не удалось подтвердить запуск через MAX. Откройте приложение заново.' : 'Не удалось связаться с сервером. Попробуйте ещё раз.')
  return response.json() as Promise<ProfileResponse>
}

export const loadProfile = (initData: string) => request(initData, 'GET')
export const saveProfile = (initData: string, body: unknown) => request(initData, 'PUT', body)

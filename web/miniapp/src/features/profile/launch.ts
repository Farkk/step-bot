export const LOCAL_PREVIEW = 'local-preview'

export function resolveLaunchData(maxInitData: string, hostname: string): string {
  if (maxInitData) return maxInitData
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]' ? LOCAL_PREVIEW : ''
}

export function taskIdFromLaunch(initData: string, search: string): string | null {
  const value = new URLSearchParams(initData).get('start_param') || new URLSearchParams(search).get('WebAppStartParam') || ''
  return /^task_(\d+)$/.exec(value)?.[1] || null
}

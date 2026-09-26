export const LOCAL_PREVIEW = 'local-preview'

export function resolveLaunchData(maxInitData: string, hostname: string): string {
  if (maxInitData) return maxInitData
  return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]' ? LOCAL_PREVIEW : ''
}

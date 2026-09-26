export type Executor = {
  id: number
  fullName: string
  phone: string
  phoneVerified: boolean
  gender: string
  age: number
  rating: number
  ratings: number
  completed: number
  registeredAt: string
}

export type ExecutorFilters = {
  search?: string
  minRating?: number
  minCompleted?: number
  gender?: string
  minAge?: number
  maxAge?: number
  phoneVerified?: boolean | null
}

export function filterExecutors(items: Executor[], filters: ExecutorFilters): Executor[] {
  const search = filters.search?.trim().toLocaleLowerCase('ru') || ''
  const digits = search.replace(/\D/g, '')
  return items.filter(item => {
    if (search && !item.fullName.toLocaleLowerCase('ru').includes(search) && !item.phone.toLocaleLowerCase('ru').includes(search) && (!digits || !item.phone.replace(/\D/g, '').includes(digits))) return false
    if (filters.minRating && (item.ratings === 0 || item.rating < filters.minRating)) return false
    if (filters.minCompleted && item.completed < filters.minCompleted) return false
    if (filters.gender && item.gender !== filters.gender) return false
    if (filters.minAge && item.age < filters.minAge) return false
    if (filters.maxAge && item.age > filters.maxAge) return false
    if (filters.phoneVerified != null && item.phoneVerified !== filters.phoneVerified) return false
    return true
  }).sort((a, b) => b.rating - a.rating || b.ratings - a.ratings || b.completed - a.completed)
}

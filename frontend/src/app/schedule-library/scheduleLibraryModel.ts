import type { Schedule } from '@/lib/backend'

export type ScheduleDateRange = 'all' | 'today' | '7d' | '30d' | '90d'
export type ScheduleStatusFilter = 'all' | 'enabled' | 'disabled'

export interface ScheduleLibraryFilter {
  search: string
  status: ScheduleStatusFilter
  category: string
  allCategories: string
  tags: string[]
  createdRange: ScheduleDateRange
  updatedRange: ScheduleDateRange
  sort: string
}

export function filterSchedules(
  schedules: Schedule[],
  filter: ScheduleLibraryFilter,
  now = new Date(),
): Schedule[] {
  const query = filter.search.trim().toLocaleLowerCase()
  const createdSince = rangeStart(filter.createdRange, now)
  const updatedSince = rangeStart(filter.updatedRange, now)
  return [...schedules]
    .filter((schedule) => {
      if (filter.status === 'enabled' && !schedule.enabled) return false
      if (filter.status === 'disabled' && schedule.enabled) return false
      if (
        filter.category !== filter.allCategories &&
        (schedule.category ?? '').toLocaleLowerCase() !== filter.category.toLocaleLowerCase()
      ) {
        return false
      }
      const scheduleTags = new Set((schedule.tags ?? []).map((tag) => tag.toLocaleLowerCase()))
      if (filter.tags.some((tag) => !scheduleTags.has(tag.toLocaleLowerCase()))) return false
      if (createdSince && Date.parse(schedule.createdAt) < createdSince) return false
      if (updatedSince && Date.parse(schedule.updatedAt) < updatedSince) return false
      if (!query) return true
      return [
        schedule.name,
        schedule.description,
        schedule.category,
        schedule.id,
        ...(schedule.tags ?? []),
      ]
        .filter(Boolean)
        .some((value) => value!.toLocaleLowerCase().includes(query))
    })
    .sort((left, right) => compareSchedules(left, right, filter.sort))
}

export function scheduleFacets(values: string[]): Array<{ value: string; count: number }> {
  const facets = new Map<string, { value: string; count: number }>()
  for (const raw of values) {
    const value = raw.trim()
    const key = value.toLocaleLowerCase()
    if (!key) continue
    const current = facets.get(key)
    if (current) current.count += 1
    else facets.set(key, { value, count: 1 })
  }
  return [...facets.values()].sort((left, right) => left.value.localeCompare(right.value))
}

export function rangeStart(range: ScheduleDateRange, current: Date): number {
  if (range === 'all') return 0
  const start = new Date(current)
  if (range === 'today') start.setHours(0, 0, 0, 0)
  else start.setDate(start.getDate() - Number.parseInt(range, 10))
  return start.getTime()
}

function compareSchedules(left: Schedule, right: Schedule, sort: string): number {
  if (sort === 'name_asc') return left.name.localeCompare(right.name)
  if (sort === 'name_desc') return right.name.localeCompare(left.name)
  if (sort === 'created_desc') return Date.parse(right.createdAt) - Date.parse(left.createdAt)
  if (sort === 'last_desc') {
    return Date.parse(right.lastFiredAt ?? '') - Date.parse(left.lastFiredAt ?? '')
  }
  return Date.parse(right.updatedAt) - Date.parse(left.updatedAt)
}

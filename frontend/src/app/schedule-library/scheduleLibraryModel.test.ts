import { describe, expect, it } from 'vitest'
import type { Schedule } from '@/lib/backend'
import { filterSchedules, scheduleFacets } from './scheduleLibraryModel'

const schedule = (id: string, overrides: Partial<Schedule> = {}) =>
  ({
    id,
    name: id,
    description: '',
    category: 'Ops',
    tags: [],
    enabled: true,
    createdAt: '2026-09-01T00:00:00Z',
    updatedAt: '2026-09-01T00:00:00Z',
    ...overrides,
  }) as Schedule

describe('scheduleLibraryModel', () => {
  it('combines status, category, tags, dates and text before sorting', () => {
    const result = filterSchedules(
      [
        schedule('older', { tags: ['Weekly'], updatedAt: '2026-09-02T00:00:00Z' }),
        schedule('newer', {
          name: 'Daily report',
          tags: ['Daily'],
          updatedAt: '2026-09-04T00:00:00Z',
        }),
        schedule('disabled', { enabled: false, tags: ['Daily'] }),
      ],
      {
        search: 'daily',
        status: 'enabled',
        category: 'ops',
        allCategories: '__all__',
        tags: ['daily'],
        createdRange: '7d',
        updatedRange: 'all',
        sort: 'updated_desc',
      },
      new Date('2026-09-05T12:00:00Z'),
    )
    expect(result.map((item) => item.id)).toEqual(['newer'])
  })

  it('merges facets case-insensitively while preserving the first label', () => {
    expect(scheduleFacets(['Daily', 'daily', '', 'Ops'])).toEqual([
      { value: 'Daily', count: 2 },
      { value: 'Ops', count: 1 },
    ])
  })
})

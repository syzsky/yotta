import { describe, expect, it, vi } from 'vitest'
import { useWorkflowLibraryQuery } from './useWorkflowLibraryQuery'

describe('useWorkflowLibraryQuery', () => {
  it('normalizes search and resets paging before reloading', async () => {
    const reload = vi.fn(async () => undefined)
    const query = useWorkflowLibraryQuery({ reload, translate: (key) => key })
    query.page.value = 4
    query.searchInput.value = '  switch  '

    await query.applySearch()

    expect(query.search.value).toBe('switch')
    expect(query.page.value).toBe(1)
    expect(reload).toHaveBeenCalledOnce()
  })

  it('builds bounded transport queries and clamps a stale page after results arrive', () => {
    const query = useWorkflowLibraryQuery({
      reload: vi.fn(async () => undefined),
      translate: (key) => key,
      now: () => new Date('2026-09-05T12:00:00.000Z'),
    })
    query.page.value = 5
    query.pageSize.value = 20
    query.categoryFilter.value = 'automation'
    query.tagFilters.value = ['desktop']
    query.createdRange.value = '7d'

    expect(query.request()).toMatchObject({
      category: 'automation',
      tags: ['desktop'],
      createdSince: '2026-08-29T12:00:00.000Z',
      page: 5,
      pageSize: 20,
    })
    expect(query.accept({ total: 21, items: [], categories: [], tags: [] } as never)).toBe(true)
    expect(query.page.value).toBe(2)
  })

  it('resets every advanced filter together', async () => {
    const query = useWorkflowLibraryQuery({
      reload: vi.fn(async () => undefined),
      translate: (key) => key,
    })
    query.categoryFilter.value = 'games'
    query.tagFilters.value = ['daily']
    query.updatedRange.value = '30d'
    query.sort.value = 'name_asc'

    await query.resetFilters()

    expect(query.hasFilters.value).toBe(false)
    expect(query.sort.value).toBe('updated_desc')
  })
})

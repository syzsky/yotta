import { describe, expect, it, vi } from 'vitest'
import type { AssetPage, AssetSummary } from '@/lib/backend'
import { useAssetLibraryBrowse } from './useAssetLibraryBrowse'

const asset = (guid: string) => ({ guid, name: guid }) as AssetSummary

describe('useAssetLibraryBrowse', () => {
  it('loads the active asset kind with normalized filters and paging', async () => {
    const queryAssets = vi.fn(
      async () =>
        ({
          items: [asset('a')],
          total: 1,
          page: 1,
          pageSize: 20,
          revision: 1,
          categories: [],
          tags: [],
        }) as AssetPage,
    )
    const browse = useAssetLibraryBrowse({
      queryAssets,
      recentGUIDs: () => ['recent'],
      translate: (key) => key,
      showError: vi.fn(),
      now: () => new Date('2026-09-05T12:00:00.000Z'),
    })
    browse.activeTab.value = 'templates'
    browse.categoryFilter.value = 'vision'
    browse.createdRange.value = '7d'
    await browse.refresh()

    expect(queryAssets).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: 'template',
        category: 'vision',
        createdSince: '2026-08-29T12:00:00.000Z',
        recentGUIDs: ['recent'],
      }),
    )
    expect(browse.assetPage.value.map((item) => item.guid)).toEqual(['a'])
    browse.activeTab.value = 'paths'
    await browse.refresh()
    expect(queryAssets).toHaveBeenLastCalledWith(
      expect.objectContaining({ kind: 'path', category: 'vision' }),
    )
  })

  it('keeps cross-page selection and retains only failed batch items', () => {
    const browse = useAssetLibraryBrowse({
      queryAssets: vi.fn(),
      recentGUIDs: () => [],
      translate: (key) => key,
      showError: vi.fn(),
    })
    browse.assetPage.value = [asset('a'), asset('b')]
    browse.toggleCurrentPage(true)
    browse.assetPage.value = [asset('c')]
    browse.toggleAsset(asset('c'), true)
    browse.retainFailedSelection(['b', 'c'])
    expect(browse.selectedRows.value.map((item) => item.guid)).toEqual(['b', 'c'])
  })
})

it('does not restore a deleted item from an older in-flight page', async () => {
  let completeOld!: (value: AssetPage) => void
  const latest = {
    items: [],
    total: 0,
    revision: 2,
    categories: [],
    tags: [],
  } as unknown as AssetPage
  const queryAssets = vi
    .fn()
    .mockImplementationOnce(
      () =>
        new Promise<AssetPage>((resolve) => {
          completeOld = resolve
        }),
    )
    .mockResolvedValueOnce(latest)
  const browse = useAssetLibraryBrowse({
    queryAssets,
    recentGUIDs: () => [],
    translate: (key) => key,
    showError: vi.fn(),
  })
  const old = browse.refresh()
  await browse.refresh()
  completeOld({ ...latest, items: [asset('deleted')], total: 1, revision: 1 })
  await old
  expect(browse.assetPage.value).toEqual([])
  expect(browse.total.value).toBe(0)
})

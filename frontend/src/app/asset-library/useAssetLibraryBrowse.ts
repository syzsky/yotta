import { computed, ref } from 'vue'
import type { AssetPage, AssetQuery, AssetSummary } from '@/lib/backend'

export type AssetLibraryTab = 'macros' | 'clips' | 'templates' | 'paths'
export type AssetDateRange = 'all' | 'today' | '7d' | '30d' | '90d'

interface AssetLibraryBrowseOptions {
  queryAssets: (query: AssetQuery) => Promise<AssetPage>
  recentGUIDs: () => string[]
  translate: (key: string, params?: Record<string, unknown>) => string
  showError: (title: string, error: unknown) => void
  now?: () => Date
}

export function useAssetLibraryBrowse(options: AssetLibraryBrowseOptions) {
  const allCategories = '__all__'
  const activeTab = ref<AssetLibraryTab>('macros')
  const queryInput = ref('')
  const query = ref('')
  const categoryFilter = ref(allCategories)
  const tagFilters = ref<string[]>([])
  const createdRange = ref<AssetDateRange>('all')
  const categories = ref<Array<{ value: string; count: number }>>([])
  const tags = ref<Array<{ value: string; count: number }>>([])
  const sort = ref('recent_desc')
  const page = ref(1)
  const pageSize = ref(20)
  const total = ref(0)
  const assetPage = ref<AssetSummary[]>([])
  const loading = ref(false)
  const selected = ref<Record<string, AssetSummary>>({})

  const selectedRows = computed(() => Object.values(selected.value))
  const hasLibraryFilters = computed(() =>
    Boolean(
      query.value ||
      categoryFilter.value !== allCategories ||
      tagFilters.value.length ||
      createdRange.value !== 'all',
    ),
  )
  const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
  const resultStart = computed(() => (total.value ? (page.value - 1) * pageSize.value + 1 : 0))
  const resultEnd = computed(() => Math.min(page.value * pageSize.value, total.value))
  const allCurrentPageSelected = computed(
    () =>
      assetPage.value.length > 0 &&
      assetPage.value.every((asset) => Boolean(selected.value[asset.guid])),
  )
  const sortItems = computed(() =>
    ['recent_desc', 'name_asc', 'name_desc', 'created_desc'].map((value) => ({
      label: options.translate(`assets.sort_${value}`),
      value,
    })),
  )
  const createdRangeItems = computed(() => [
    { label: options.translate('assets.created_any'), value: 'all' },
    { label: options.translate('assets.created_today'), value: 'today' },
    { label: options.translate('assets.created_days', { n: 7 }), value: '7d' },
    { label: options.translate('assets.created_days', { n: 30 }), value: '30d' },
    { label: options.translate('assets.created_days', { n: 90 }), value: '90d' },
  ])
  const pageSizeItems = [20, 50, 100].map((value) => ({ label: String(value), value }))
  const categoryFilterItems = computed(() => [
    { label: options.translate('assets.all_categories'), value: allCategories },
    ...categories.value.map((item) => ({
      label: `${item.value} (${item.count})`,
      value: item.value,
    })),
  ])
  const tagOptions = computed(() => tags.value.map((item) => item.value))

  let requestGeneration = 0
  async function refresh(): Promise<void> {
    const generation = ++requestGeneration
    loading.value = true
    try {
      const result = await options.queryAssets({
        search: query.value,
        kind:
          activeTab.value === 'macros'
            ? 'macro'
            : activeTab.value === 'clips'
              ? 'clip'
              : activeTab.value === 'paths'
                ? 'path'
                : 'template',
        category: categoryFilter.value === allCategories ? '' : categoryFilter.value.trim(),
        tags: [...tagFilters.value],
        createdSince: rangeStart(createdRange.value),
        sort: sort.value,
        page: page.value,
        pageSize: pageSize.value,
        thumbnailBudget: pageSize.value,
        recentGUIDs: [...options.recentGUIDs()],
      })
      if (generation !== requestGeneration) return
      assetPage.value = result.items ?? []
      total.value = result.total ?? 0
      categories.value = result.categories ?? []
      tags.value = result.tags ?? []
      if (page.value > pageCount.value) {
        page.value = pageCount.value
        await refresh()
      }
    } catch (error) {
      if (generation !== requestGeneration) return
      options.showError(options.translate('assets.load_failed'), error)
    } finally {
      if (generation === requestGeneration) loading.value = false
    }
  }

  async function applyQuery(): Promise<void> {
    query.value = queryInput.value.trim()
    page.value = 1
    await refresh()
  }

  async function changeQuery(): Promise<void> {
    page.value = 1
    await refresh()
  }

  async function resetLibraryFilters(): Promise<void> {
    queryInput.value = ''
    query.value = ''
    categoryFilter.value = allCategories
    tagFilters.value = []
    createdRange.value = 'all'
    await changeQuery()
  }

  async function goToPage(next: number): Promise<void> {
    if (next < 1 || next > pageCount.value || next === page.value) return
    page.value = next
    await refresh()
  }

  function toggleAsset(asset: AssetSummary, checked: boolean): void {
    const next = { ...selected.value }
    if (checked) next[asset.guid] = asset
    else delete next[asset.guid]
    selected.value = next
  }

  function toggleCurrentPage(checked: boolean): void {
    const next = { ...selected.value }
    for (const asset of assetPage.value) {
      if (checked) next[asset.guid] = asset
      else delete next[asset.guid]
    }
    selected.value = next
  }

  function clearSelection(): void {
    selected.value = {}
  }

  function retainFailedSelection(guids: string[]): void {
    const failed = new Set(guids)
    selected.value = Object.fromEntries(
      selectedRows.value
        .filter((asset) => failed.has(asset.guid))
        .map((asset) => [asset.guid, asset]),
    )
  }

  function rangeStart(range: AssetDateRange): string {
    if (range === 'all') return ''
    const start = options.now?.() ?? new Date()
    if (range === 'today') start.setHours(0, 0, 0, 0)
    else start.setDate(start.getDate() - Number.parseInt(range, 10))
    return start.toISOString()
  }

  return {
    activeTab,
    queryInput,
    query,
    categoryFilter,
    tagFilters,
    createdRange,
    categories,
    tags,
    sort,
    page,
    pageSize,
    total,
    assetPage,
    loading,
    selected,
    selectedRows,
    hasLibraryFilters,
    pageCount,
    resultStart,
    resultEnd,
    allCurrentPageSelected,
    sortItems,
    createdRangeItems,
    pageSizeItems,
    categoryFilterItems,
    tagOptions,
    refresh,
    applyQuery,
    changeQuery,
    resetLibraryFilters,
    goToPage,
    toggleAsset,
    toggleCurrentPage,
    clearSelection,
    retainFailedSelection,
  }
}

import { computed, ref } from 'vue'
import type { SourcePage, SourceQuery } from '@/app/transport/workflow'

export type WorkflowDateRange = 'all' | 'today' | '7d' | '30d' | '90d'

interface WorkflowLibraryQueryOptions {
  reload: () => Promise<void>
  translate: (key: string, params?: Record<string, unknown>) => string
  now?: () => Date
}

export function useWorkflowLibraryQuery(options: WorkflowLibraryQueryOptions) {
  const allCategories = '__all__'
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(20)
  const sort = ref('updated_desc')
  const searchInput = ref('')
  const search = ref('')
  const categoryFilter = ref(allCategories)
  const tagFilters = ref<string[]>([])
  const createdRange = ref<WorkflowDateRange>('all')
  const updatedRange = ref<WorkflowDateRange>('all')
  const categories = ref<Array<{ value: string; count: number }>>([])
  const tags = ref<Array<{ value: string; count: number }>>([])

  const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
  const resultStart = computed(() => (total.value ? (page.value - 1) * pageSize.value + 1 : 0))
  const resultEnd = computed(() => Math.min(page.value * pageSize.value, total.value))
  const hasAdvancedFilters = computed(() =>
    Boolean(
      categoryFilter.value !== allCategories ||
      tagFilters.value.length ||
      createdRange.value !== 'all' ||
      updatedRange.value !== 'all',
    ),
  )
  const hasFilters = computed(() => Boolean(search.value || hasAdvancedFilters.value))
  const categoryFilterItems = computed(() => [
    { label: options.translate('workflow.list.all_categories'), value: allCategories },
    ...categories.value.map((item) => ({
      label: `${item.value} (${item.count})`,
      value: item.value,
    })),
  ])
  const tagOptions = computed(() => tags.value.map((item) => item.value))
  const createdRangeItems = computed(() => dateRangeItems('created'))
  const updatedRangeItems = computed(() => dateRangeItems('updated'))
  const sortItems = computed(() =>
    ['name_asc', 'name_desc', 'nodes_desc', 'revision_desc', 'created_desc', 'updated_desc'].map(
      (value) => ({ label: options.translate(`workflow.list.sort_${value}`), value }),
    ),
  )
  const pageSizeItems = [20, 50, 100].map((value) => ({ label: String(value), value }))

  function request(): SourceQuery {
    return {
      search: search.value,
      category: categoryFilter.value === allCategories ? '' : categoryFilter.value,
      tags: [...tagFilters.value],
      createdSince: rangeStart(createdRange.value),
      updatedSince: rangeStart(updatedRange.value),
      sort: sort.value,
      page: page.value,
      pageSize: pageSize.value,
    } as SourceQuery
  }

  function accept(result: SourcePage): boolean {
    total.value = result.total
    categories.value = result.categories ?? []
    tags.value = result.tags ?? []
    if (page.value <= pageCount.value) return false
    page.value = pageCount.value
    return true
  }

  async function queryChanged(): Promise<void> {
    page.value = 1
    await options.reload()
  }

  async function applySearch(): Promise<void> {
    search.value = searchInput.value.trim()
    await queryChanged()
  }

  async function resetFilters(): Promise<void> {
    searchInput.value = ''
    search.value = ''
    categoryFilter.value = allCategories
    tagFilters.value = []
    createdRange.value = 'all'
    updatedRange.value = 'all'
    sort.value = 'updated_desc'
    await queryChanged()
  }

  async function goToPage(next: number): Promise<void> {
    if (next < 1 || next > pageCount.value || next === page.value) return
    page.value = next
    await options.reload()
  }

  function dateRangeItems(kind: 'created' | 'updated') {
    const prefix = kind === 'created' ? 'created' : 'updated'
    return [
      { label: options.translate(`workflow.list.${prefix}_any`), value: 'all' },
      { label: options.translate(`workflow.list.${prefix}_today`), value: 'today' },
      { label: options.translate(`workflow.list.${prefix}_days`, { n: 7 }), value: '7d' },
      { label: options.translate(`workflow.list.${prefix}_days`, { n: 30 }), value: '30d' },
      { label: options.translate(`workflow.list.${prefix}_days`, { n: 90 }), value: '90d' },
    ]
  }

  function rangeStart(range: WorkflowDateRange): string {
    if (range === 'all') return ''
    const start = options.now?.() ?? new Date()
    if (range === 'today') start.setHours(0, 0, 0, 0)
    else start.setDate(start.getDate() - Number.parseInt(range, 10))
    return start.toISOString()
  }

  return {
    total,
    page,
    pageSize,
    sort,
    searchInput,
    search,
    categoryFilter,
    tagFilters,
    createdRange,
    updatedRange,
    categories,
    tags,
    pageCount,
    resultStart,
    resultEnd,
    hasAdvancedFilters,
    hasFilters,
    categoryFilterItems,
    tagOptions,
    createdRangeItems,
    updatedRangeItems,
    sortItems,
    pageSizeItems,
    request,
    accept,
    queryChanged,
    applySearch,
    resetFilters,
    goToPage,
  }
}

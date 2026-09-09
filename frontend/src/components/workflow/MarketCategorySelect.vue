<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ensureWorkflowIcons } from '@/lib/workflowIcons'
import { useI18n } from 'vue-i18n'
import { categoryRows, type MarketCategory } from '@/lib/marketCategories'

defineOptions({ inheritAttrs: false })
const props = defineProps<{
  categories: MarketCategory[]
  allLabel?: string
  placeholder?: string
  disabled?: boolean
  activeOnly?: boolean
}>()
const model = defineModel<string>({ required: true })
const { t } = useI18n()
const open = ref(false)
const search = ref('')
const expanded = ref<string[]>([])
watch(
  () => props.categories,
  (categories) => {
    void ensureWorkflowIcons(categories.map((item) => item.icon || 'i-tabler-folder')).catch(
      () => {},
    )
  },
  { immediate: true },
)
const rows = computed(() => categoryRows(props.categories))
type Item = {
  key: string
  label: string
  path: string
  active: boolean
  icon?: string
  children?: Item[]
}
const items = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  const allowed = new Set(
    rows.value.filter((r) => !props.activeOnly || r.active).flatMap((r) => [r.key, ...r.ancestors]),
  )
  const entries = rows.value.filter((r) => allowed.has(r.key))
  const result: Item[] = []
  if (props.allLabel && (!query || props.allLabel.toLocaleLowerCase().includes(query))) {
    result.push({ key: '', label: props.allLabel, path: props.allLabel, active: true })
  }
  const byKey = new Map<string, Item>()
  for (const row of entries) {
    if (
      query &&
      (!row.label.toLocaleLowerCase().includes(query) || (props.activeOnly && !row.active))
    )
      continue
    const item: Item = {
      key: row.key,
      label: query ? row.label : row.name,
      path: row.label,
      active: row.active,
      icon: row.icon,
    }
    const parent = query ? undefined : byKey.get(row.parentKey)
    byKey.set(item.key, item)
    if (parent) (parent.children ||= []).push(item)
    else result.push(item)
  }
  return result
})
const currentCategory = computed(() => props.categories.find((item) => item.key === model.value))
const label = computed(
  () =>
    rows.value.find((r) => r.key === model.value)?.label ||
    model.value ||
    props.allLabel ||
    props.placeholder,
)
const selected = computed(() => ({
  key: model.value,
  label: label.value || '',
  path: label.value || '',
  active: true,
}))
watch(open, (value) => {
  if (value) {
    search.value = ''
    expanded.value = rows.value.map((r) => r.key)
  }
})
function select(event: Event, item: Item) {
  event.preventDefault()
  if (props.activeOnly && !item.active) return
  model.value = item.key
  open.value = false
}
</script>

<template>
  <UPopover v-model:open="open" :content="{ align: 'start' }">
    <UButton
      v-bind="$attrs"
      color="neutral"
      variant="outline"
      :disabled="disabled"
      trailing-icon="i-tabler-chevron-down"
      class="justify-between"
      :leading-icon="currentCategory?.icon || (model ? 'i-tabler-folder' : undefined)"
      :title="label"
      :aria-label="String($attrs['aria-label'] || placeholder || allLabel || '')"
      :ui="{ label: 'truncate', trailingIcon: 'shrink-0' }"
      >{{ label }}</UButton
    >
    <template #content>
      <div class="w-72 max-w-[calc(100vw-2rem)] p-1.5">
        <UInput
          v-model="search"
          autofocus
          icon="i-tabler-search"
          :placeholder="t('common.search_options')"
          :aria-label="t('common.search_options')"
          class="mb-1.5 w-full"
          @keydown.escape="open = false"
        />
        <UTree
          v-if="items.length"
          v-model:expanded="expanded"
          :items="items"
          :model-value="selected"
          :get-key="(item: Item) => item.key"
          :as="{ root: 'ul', link: 'div' }"
          class="max-h-72 overflow-y-auto"
          @select="select"
        >
          <template #item-leading="{ item }"
            ><UIcon v-if="item.key" :name="item.icon || 'i-tabler-folder'" class="size-4 shrink-0"
          /></template>
          <template #item-label="{ item }">
            <span :title="item.path" :class="{ 'opacity-50': activeOnly && !item.active }">{{
              item.label
            }}</span>
          </template>
          <template #item-trailing="{ item, expanded: isExpanded, handleToggle }">
            <UIcon v-if="item.key === model" name="i-tabler-check" class="size-4 text-primary" />
            <UButton
              v-if="item.children?.length"
              :icon="isExpanded ? 'i-tabler-chevron-down' : 'i-tabler-chevron-right'"
              color="neutral"
              variant="ghost"
              size="xs"
              :aria-label="item.path"
              :aria-expanded="isExpanded"
              @click.stop="handleToggle"
            />
          </template>
        </UTree>
        <p v-else class="p-3 text-sm text-muted">{{ t('workflow.market.category_empty') }}</p>
      </div>
    </template>
  </UPopover>
</template>

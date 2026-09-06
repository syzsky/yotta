<template>
  <div class="min-w-0" data-testid="arrangement-table">
    <div class="overflow-x-auto rounded-lg border border-default">
      <div class="relative" :style="{ minWidth: minWidth ?? '680px' }">
        <div
          v-if="selected.length"
          class="absolute inset-x-0 top-0 z-10 flex h-10 items-center gap-2 whitespace-nowrap bg-elevated px-2"
          data-testid="arrangement-bulk"
        >
          <span class="mr-1 text-xs font-medium text-primary">{{
            t('rowEditor.selected', { count: selected.length })
          }}</span>
          <UButton
            size="xs"
            color="neutral"
            variant="ghost"
            :label="t('rowEditor.select_all')"
            :disabled="allSelected"
            @click="
              emit(
                'update:selection',
                modelValue.map((row) => row.id),
              )
            "
          /><slot name="bulk" :ids="selected" />
          <UPopover :ui="{ content: 'w-80 p-3' }"
            ><UButton
              size="xs"
              variant="soft"
              color="neutral"
              icon="i-tabler-arrows-sort"
              :label="t('rowEditor.move')"
            />
            <template #content
              ><div class="space-y-2">
                <p class="text-xs font-medium">{{ t('rowEditor.move') }}</p>
                <AdaptiveSelect
                  :model-value="''"
                  :items="moveOptions"
                  width-mode="fill"
                  :placeholder="t('rowEditor.destination')"
                  :aria-label="t('rowEditor.destination')"
                  @update:model-value="move(selected, String($event))"
                /></div
            ></template>
          </UPopover>
          <UDropdownMenu :items="bulkMenu"
            ><UButton
              size="xs"
              color="neutral"
              variant="ghost"
              icon="i-tabler-dots"
              :aria-label="t('rowEditor.more_selected')"
          /></UDropdownMenu>
          <UButton
            class="ml-auto"
            size="xs"
            color="neutral"
            variant="ghost"
            :label="t('rowEditor.clear_selection')"
            @click="emit('update:selection', [])"
          />
        </div>
        <table
          class="w-full border-collapse text-left text-xs"
          :style="{ minWidth: minWidth ?? '680px' }"
        >
          <thead
            class="h-10 bg-elevated text-muted"
            :class="{ invisible: selected.length > 0 }"
            :aria-hidden="selected.length > 0"
          >
            <tr>
              <th scope="col" class="w-9 px-2 py-2">
                <UCheckbox
                  :model-value="allSelected ? true : selected.length ? 'indeterminate' : false"
                  :aria-label="t('rowEditor.select_all')"
                  @update:model-value="
                    emit('update:selection', $event === true ? modelValue.map((row) => row.id) : [])
                  "
                />
              </th>
              <th scope="col" class="w-8">
                <span class="sr-only">{{ t('rowEditor.order') }}</span>
              </th>
              <th scope="col" class="w-8 px-1 py-2 font-normal">#</th>
              <th
                v-for="column in columns"
                :key="column.key"
                scope="col"
                class="px-2 py-2 font-medium"
                :style="{ width: column.width }"
              >
                {{ column.label }}
              </th>
              <th scope="col" class="w-10">
                <span class="sr-only">{{ t('rowEditor.more') }}</span>
              </th>
            </tr>
          </thead>
          <VueDraggable
            v-model="rows"
            tag="tbody"
            :animation="150"
            handle=".arrangement-grip"
            ghost-class="arrangement-ghost"
            @start="startDrag"
            @end="endDrag"
          >
            <tr
              v-for="(row, index) in rows"
              :key="row.id"
              :data-row-id="row.id"
              class="border-t border-default"
              :class="selected.includes(row.id) ? 'bg-primary/5' : 'hover:bg-elevated/40'"
              @focusin="emit('activate', row.id)"
            >
              <td class="px-2 py-2">
                <UCheckbox
                  :model-value="selected.includes(row.id)"
                  :aria-label="t('rowEditor.select_row', { n: index + 1 })"
                  @update:model-value="toggle(row.id, $event === true)"
                />
              </td>
              <td>
                <UButton
                  size="xs"
                  color="neutral"
                  variant="ghost"
                  icon="i-tabler-grip-vertical"
                  class="arrangement-grip cursor-grab active:cursor-grabbing"
                  :aria-label="t('rowEditor.drag')"
                  :title="t('rowEditor.drag')"
                  aria-keyshortcuts="Alt+ArrowUp Alt+ArrowDown"
                  @keydown.alt.up.stop.prevent="step(row.id, -1)"
                  @keydown.alt.down.stop.prevent="step(row.id, 1)"
                />
              </td>
              <td class="px-1 py-2 tabular-nums text-dimmed">{{ index + 1 }}</td>
              <slot name="cells" :row="row" :index="index" />
              <td class="px-1 py-2">
                <UDropdownMenu :items="rowMenu(row.id, index)"
                  ><UButton
                    size="xs"
                    color="neutral"
                    variant="ghost"
                    icon="i-tabler-dots-vertical"
                    :aria-label="t('rowEditor.more_row', { n: index + 1 })"
                /></UDropdownMenu>
              </td>
            </tr>
          </VueDraggable>
        </table>
      </div>
    </div>
  </div>
</template>
<script setup lang="ts" generic="T extends OrderedRow">
import { computed, nextTick, watch } from 'vue'
import { VueDraggable } from 'vue-draggable-plus'
import { useI18n } from 'vue-i18n'
import AdaptiveSelect from '@/components/common/AdaptiveSelect.vue'
import { applyRowDrop, moveRowsBefore, moveRowsOneStep, type OrderedRow } from './ordering'
const props = defineProps<{
  modelValue: T[]
  selection: string[]
  columns: Array<{ key: string; label: string; width?: string }>
  rowLabel: (row: T) => string
  minWidth?: string
}>()
const emit = defineEmits<{
  'update:modelValue': [T[]]
  'update:selection': [string[]]
  change: []
  remove: [string[]]
  activate: [string]
}>()
defineSlots<{
  cells: (props: { row: T; index: number }) => unknown
  bulk?: (props: { ids: string[] }) => unknown
}>()
const { t } = useI18n()
const selected = computed(() =>
  props.selection.filter((id) => props.modelValue.some((row) => row.id === id)),
)
const allSelected = computed(
  () => props.modelValue.length > 0 && selected.value.length === props.modelValue.length,
)
const rows = computed({
  get: () => props.modelValue,
  set: (value: T[]) => emit('update:modelValue', value),
})
watch(
  () => props.modelValue.map((row) => row.id).join('\0'),
  () => {
    if (selected.value.length !== props.selection.length) emit('update:selection', selected.value)
  },
)
function toggle(id: string, on: boolean) {
  emit(
    'update:selection',
    on ? [...new Set([...selected.value, id])] : selected.value.filter((value) => value !== id),
  )
}
function commit(value: T[]) {
  emit('update:modelValue', value)
  emit('change')
}
function move(ids: string[], target: string) {
  const anchor =
    target === 'edge:bottom'
      ? null
      : target === 'edge:top'
        ? (props.modelValue.find((row) => !ids.includes(row.id))?.id ?? null)
        : target
  commit(moveRowsBefore(props.modelValue, ids, anchor))
}
function step(id: string, direction: -1 | 1) {
  commit(
    moveRowsOneStep(
      props.modelValue,
      selected.value.includes(id) ? selected.value : [id],
      direction,
    ),
  )
}
function rowMenu(id: string, index: number) {
  return [
    [
      {
        label: t('rowEditor.top'),
        icon: 'i-tabler-arrow-bar-to-up',
        disabled: index === 0,
        onSelect: () => move([id], 'edge:top'),
      },
      {
        label: t('rowEditor.bottom'),
        icon: 'i-tabler-arrow-bar-to-down',
        disabled: index === props.modelValue.length - 1,
        onSelect: () => move([id], 'edge:bottom'),
      },
    ],
    [
      {
        label: t('rowEditor.delete'),
        icon: 'i-tabler-trash',
        color: 'error' as const,
        onSelect: () => emit('remove', [id]),
      },
    ],
  ]
}
const bulkMenu = computed(() => [
  [
    {
      label: t('rowEditor.top'),
      icon: 'i-tabler-arrow-bar-to-up',
      onSelect: () => move(selected.value, 'edge:top'),
    },
    {
      label: t('rowEditor.bottom'),
      icon: 'i-tabler-arrow-bar-to-down',
      onSelect: () => move(selected.value, 'edge:bottom'),
    },
  ],
  [
    {
      label: t('rowEditor.delete_selected', { count: selected.value.length }),
      icon: 'i-tabler-trash',
      color: 'error' as const,
      onSelect: () => emit('remove', selected.value),
    },
  ],
])
const moveOptions = computed(() => [
  { label: t('rowEditor.top'), value: 'edge:top' },
  { label: t('rowEditor.bottom'), value: 'edge:bottom' },
  ...props.modelValue.flatMap((row, index) =>
    selected.value.includes(row.id)
      ? []
      : [
          {
            value: row.id,
            label: t('rowEditor.before', { n: index + 1, name: props.rowLabel(row) }),
          },
        ],
  ),
])
let before: T[] = []
let draggedId = ''
let moving: string[] = []
function startDrag(event: { oldIndex?: number }) {
  before = [...props.modelValue]
  draggedId = before[event.oldIndex ?? -1]?.id ?? ''
  moving = selected.value.includes(draggedId) ? [...selected.value] : [draggedId]
  if (draggedId && !selected.value.includes(draggedId)) emit('update:selection', [draggedId])
}
async function endDrag(event: { newIndex?: number }) {
  const origin = before,
    id = draggedId,
    selection = moving
  before = []
  draggedId = ''
  moving = []
  if (!id || event.newIndex === undefined) return
  await nextTick()
  commit(applyRowDrop(origin, props.modelValue, id, event.newIndex, selection))
}
</script>
<style scoped>
.arrangement-ghost {
  opacity: 0.35;
}
</style>

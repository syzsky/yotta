<template>
  <main class="panel-manager flex h-full min-h-0 flex-col bg-default" data-testid="panel-manager">
    <header class="flex items-center justify-between gap-3 border-b border-default px-6 py-4">
      <h1 class="text-lg font-semibold text-highlighted">{{ t('panels.manager') }}</h1>
      <UButton size="sm" variant="soft" icon="i-tabler-plus" :disabled="busy" @click="create">{{
        t('panels.create')
      }}</UButton>
    </header>
    <UAlert
      v-if="failure || catalogFailure"
      class="mx-6 mt-4"
      color="error"
      :title="failure || catalogFailure"
    />
    <div
      class="grid min-h-0 flex-1 grid-cols-1 overflow-auto md:grid-cols-[220px_minmax(0,1fr)] md:overflow-hidden"
    >
      <aside
        class="space-y-5 border-b border-default p-4 md:overflow-auto md:border-b-0 md:border-r"
      >
        <section v-for="owned in [true, false]" :key="String(owned)">
          <h2 class="mb-2 px-2 text-xs font-medium text-muted">
            {{ t(owned ? 'panels.mine' : 'panels.from_plugins') }}
          </h2>
          <p v-if="!items.some((p) => p.managed === owned)" class="px-2 py-2 text-xs text-muted">
            {{ t('panels.none') }}
          </p>
          <button
            v-for="item in items.filter((p) => p.managed === owned)"
            :key="item.id"
            type="button"
            class="flex w-full items-center gap-2 rounded-md px-3 py-2.5 text-left text-sm hover:bg-elevated focus-visible:outline-2 focus-visible:outline-primary"
            :class="selected === item.id ? 'bg-elevated text-primary' : 'text-toned'"
            @click="select(item)"
          >
            <UIcon name="i-tabler-layout-dashboard" class="size-4 shrink-0" /><span class="truncate"
              >{{ label(item.definition.titleKey)
              }}{{
                items.filter((x) => x.definition.titleKey === item.definition.titleKey).length > 1
                  ? ' · ' + item.id.slice(-6)
                  : ''
              }}</span
            >
          </button>
        </section>
      </aside>
      <div class="min-w-0 md:overflow-auto">
        <div class="settings-page">
          <template v-if="draft || source">
            <SettingsSection
              :title="t('panels.configuration')"
              icon="i-tabler-adjustments-horizontal"
            >
              <template #actions
                ><div class="flex flex-wrap gap-2">
                  <UButton
                    v-if="source"
                    size="sm"
                    color="neutral"
                    variant="soft"
                    icon="i-tabler-external-link"
                    :disabled="busy"
                    @click="show"
                    >{{ t('panels.show') }}</UButton
                  >
                  <template v-if="draft">
                    <UButton
                      v-if="dirty"
                      size="sm"
                      color="neutral"
                      variant="ghost"
                      @click="discard"
                      >{{ t('panels.discard') }}</UButton
                    >
                    <UButton
                      size="sm"
                      variant="soft"
                      :loading="busy"
                      :disabled="!dirty || !draft.components.length"
                      @click="save"
                      >{{ t('panels.save') }}</UButton
                    >
                    <UButton
                      v-if="draft.id"
                      size="sm"
                      color="error"
                      variant="ghost"
                      :disabled="busy"
                      @click="remove"
                      >{{ t(confirmDelete ? 'panels.confirm_delete' : 'panels.delete') }}</UButton
                    >
                  </template>
                </div></template
              >
              <div class="grid items-center gap-4 sm:grid-cols-2">
                <UFormField :label="t('panels.name')"
                  ><UInput
                    v-if="draft"
                    v-model="draft.title"
                    class="w-full"
                    size="sm"
                    :maxlength="128"
                    :aria-label="t('panels.name')"
                  />
                  <p v-else class="text-sm text-toned">
                    {{ label(source?.definition.titleKey ?? '') }}
                  </p></UFormField
                >
                <div v-if="draft?.id || source?.id" class="flex items-center gap-2">
                  <span class="text-xs text-muted">{{ t('panels.panel_id') }}</span>
                  <ReferenceIdCell
                    :id="draft?.id || source?.id || ''"
                    :label="t('panels.panel_id')"
                  />
                </div>
                <p v-else class="text-xs text-muted">{{ t('panels.id_after_save') }}</p>
              </div>
              <p v-if="!draft" class="text-xs text-muted">{{ t('panels.plugin_definition') }}</p>
            </SettingsSection>
            <SettingsSection
              v-if="draft"
              :title="t('panels.components')"
              icon="i-tabler-components"
            >
              <template #badge
                ><UBadge size="xs" color="neutral" variant="subtle">{{
                  draft.components.length
                }}</UBadge></template
              >
              <template #actions
                ><div class="flex shrink-0 items-center gap-2">
                  <AdaptiveSelect
                    v-model="newKind"
                    width-mode="content"
                    :min-width="14"
                    size="sm"
                    :items="kindItems"
                    :aria-label="t('panels.component_kind')"
                  /><UButton
                    class="shrink-0 whitespace-nowrap"
                    size="sm"
                    variant="soft"
                    icon="i-tabler-plus"
                    @click="addComponent"
                    >{{ t('panels.add_component') }}</UButton
                  >
                </div></template
              >
              <p v-if="!draft.components.length" class="text-sm text-muted">
                {{ t('panels.add_hint') }}
              </p>
              <ArrangementTable
                v-else
                v-model="draft.components"
                v-model:selection="selectedComponents"
                :columns="componentColumns"
                :row-label="componentLabel"
                min-width="740px"
                @remove="removeComponents"
              >
                <template #bulk
                  ><BatchEditPopover
                    :fields="componentBatchFields"
                    :count="selectedComponents.length"
                    @apply="applyComponentBatch"
                /></template>
                <template #cells="{ row: c, index }">
                  <td class="px-2 py-2">
                    <InlineIconPicker v-model="c.icon" fallback="i-tabler-box" />
                  </td>
                  <td class="px-2 py-2">
                    <UInput
                      v-model="c.title"
                      size="xs"
                      class="w-full"
                      :aria-label="t('rowEditor.row_name', { n: index + 1 })"
                    />
                  </td>
                  <td class="px-2 py-2 whitespace-nowrap text-muted">
                    {{ t(`panels.kind.${c.kind}`) }}
                  </td>
                  <td class="px-2 py-2">
                    <USwitch
                      v-if="['toggle', 'status'].includes(c.kind)"
                      :model-value="Boolean(c.initial)"
                      :aria-label="t('panels.initial_value')"
                      @update:model-value="c.initial = $event"
                    />
                    <UInput
                      v-else-if="['number', 'timer', 'progress'].includes(c.kind)"
                      type="number"
                      size="xs"
                      class="w-full"
                      :model-value="Number(c.initial)"
                      :aria-label="t('panels.initial_value')"
                      @update:model-value="c.initial = Number($event)"
                    />
                    <div v-else-if="c.kind === 'select'" class="flex items-center gap-1">
                      <AdaptiveSelect
                        v-model="c.initial"
                        :items="c.options ?? []"
                        width-mode="fill"
                        size="xs"
                        :aria-label="t('panels.initial_value')"
                      /><UPopover :ui="{ content: 'w-80 p-3' }"
                        ><UButton
                          size="xs"
                          color="neutral"
                          variant="ghost"
                          icon="i-tabler-list-details"
                          :aria-label="t('panels.choices_lines')" /><template #content
                          ><UFormField :label="t('panels.choices_lines')"
                            ><UTextarea
                              :model-value="(c.options ?? []).join('\n')"
                              class="w-full"
                              @update:model-value="
                                c.options = String($event).split('\n')
                              " /></UFormField></template
                      ></UPopover>
                    </div>
                    <UInput
                      v-else-if="!['button', 'log'].includes(c.kind)"
                      v-model="c.initial"
                      size="xs"
                      class="w-full"
                      :aria-label="t('panels.initial_value')"
                    />
                    <span v-else class="text-dimmed">—</span>
                  </td>
                  <td class="px-1 py-2">
                    <ReferenceIdCell :id="c.id" :label="t('panels.component_id')" />
                  </td>
                </template>
              </ArrangementTable>
            </SettingsSection>
            <section class="settings-section" data-testid="panel-preview-card">
              <UCollapsible v-model:open="previewOpen">
                <UButton
                  color="neutral"
                  variant="ghost"
                  size="sm"
                  icon="i-tabler-eye"
                  :trailing-icon="previewOpen ? 'i-tabler-chevron-up' : 'i-tabler-chevron-down'"
                  :label="t('panels.preview')"
                />
                <template #content
                  ><div class="space-y-4 p-4">
                    <p class="text-xs text-muted">{{ t('panels.preview_hint') }}</p>
                    <PanelRenderer
                      :components="previewComponents"
                      :snapshot="previewSnapshot"
                      :disabled="true"
                      :busy="''"
                      :now="Date.now()"
                    /></div
                ></template>
              </UCollapsible>
            </section>
          </template>
          <div v-else class="settings-empty-state">
            <UIcon name="i-tabler-layout-dashboard" class="size-8 text-muted" />
            <h2 class="text-base font-medium">{{ t('panels.manager_empty') }}</h2>
            <p class="max-w-md text-sm text-muted">{{ t('panels.manager_empty_hint') }}</p>
            <UButton variant="soft" @click="create">{{ t('panels.create') }}</UButton>
          </div>
        </div>
      </div>
    </div>
  </main>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  panelBackend,
  type PanelDraft,
  type PanelSource,
  type PanelComponent,
  type PanelSnapshot,
} from '@/lib/panels'
import { usePanelCatalog } from '@/composables/usePanelCatalog'
import { errorMessage } from '@/lib/invoke'
import SettingsSection from '@/components/settings/SettingsSection.vue'
import ReferenceIdCell from '@/components/arrangement/ReferenceIdCell.vue'
import ArrangementTable from '@/components/arrangement/ArrangementTable.vue'
import BatchEditPopover from '@/components/arrangement/BatchEditPopover.vue'
import InlineIconPicker from '@/components/arrangement/InlineIconPicker.vue'
import AdaptiveSelect from '@/components/common/AdaptiveSelect.vue'
import type { BatchField } from '@/components/arrangement/batchFields'
import PanelRenderer from '@/components/panels/PanelRenderer.vue'
const { t, te } = useI18n()
const label = (s: string) => (te(s) ? t(s) : s)
const { items, failure: catalogFailure, refresh } = usePanelCatalog()
const selectedComponents = ref<string[]>([])
const previewOpen = ref(true)
const selected = ref(''),
  draft = ref<PanelDraft | null>(null),
  baseline = ref(''),
  failure = ref(''),
  busy = ref(false),
  confirmDelete = ref(false),
  newKind = ref('number')
const source = computed(() => items.value.find((p) => p.id === selected.value))
const dirty = computed(() => draft.value !== null && JSON.stringify(draft.value) !== baseline.value)
const kindItems = computed(() =>
  [
    'number',
    'text',
    'status',
    'toggle',
    'input',
    'select',
    'button',
    'log',
    'progress',
    'timer',
  ].map((value) => ({ value, label: t(`panels.kind.${value}`) })),
)
const previewComponents = computed<PanelComponent[]>(() =>
  draft.value
    ? draft.value.components.map((c) => ({
        id: c.id,
        kind: c.kind,
        icon: c.icon,
        titleKey: c.title,
        field: c.id,
        precision: 2,
        options: c.options?.map((o) => ({ value: o, labelKey: o })),
      }))
    : (source.value?.definition.components ?? []),
)
const previewSnapshot = computed<PanelSnapshot>(() => ({
  protocol: 'yotta.panel-provider/v1',
  sessionId: 'preview',
  revision: 0,
  status: 'ready',
  values: Object.fromEntries((draft.value?.components ?? []).map((c) => [c.id, c.initial ?? ''])),
  records: {},
  controlRevisions: {},
}))
function canLeave() {
  if (dirty.value) {
    failure.value = t('panels.unsaved')
    return false
  }
  return true
}
onBeforeRouteLeave(() => canLeave())
function create() {
  if (!canLeave()) return
  selectedComponents.value = []
  previewOpen.value = true
  selected.value = ''
  draft.value = { id: '', revision: 0, title: t('panels.new_name'), components: [] }
  baseline.value = ''
  failure.value = ''
  confirmDelete.value = false
}
async function select(item: PanelSource) {
  if (!canLeave()) return
  busy.value = true
  try {
    draft.value = item.managed ? await panelBackend.edit(item.id) : null
    selectedComponents.value = []
    previewOpen.value = true
    selected.value = item.id
    baseline.value = JSON.stringify(draft.value)
    failure.value = ''
    confirmDelete.value = false
  } catch (e) {
    failure.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
function addComponent() {
  if (!draft.value) return
  const kind = newKind.value
  const id = 'c-' + crypto.randomUUID()
  draft.value.components.push({
    id,
    kind,
    title: t(`panels.kind.${kind}`),
    initial: ['number', 'timer', 'progress'].includes(kind)
      ? 0
      : ['toggle', 'status'].includes(kind)
        ? false
        : '',
    ...(kind === 'select'
      ? { options: [t('panels.continue'), t('panels.stop')], initial: t('panels.continue') }
      : {}),
  })
}
const componentColumns = computed(() => [
  { key: 'icon', label: t('rowEditor.icon'), width: '40px' },
  { key: 'title', label: t('rowEditor.name'), width: '28%' },
  { key: 'kind', label: t('rowEditor.type'), width: '68px' },
  { key: 'initial', label: t('panels.initial_value') },
  { key: 'id', label: t('rowEditor.id'), width: '96px' },
])
const componentLabel = (c: PanelDraft['components'][number]) => c.title || t('rowEditor.unnamed')
function removeComponents(ids: string[]) {
  if (!draft.value) return
  draft.value.components = draft.value.components.filter((c) => !ids.includes(c.id))
  selectedComponents.value = selectedComponents.value.filter((id) => !ids.includes(id))
}
function valueKind(kind: string) {
  return ['toggle', 'status'].includes(kind)
    ? 'boolean'
    : ['number', 'timer', 'progress'].includes(kind)
      ? 'number'
      : ['text', 'input', 'select'].includes(kind)
        ? 'text'
        : null
}
const componentBatchFields = computed<BatchField[]>(() => {
  const items = draft.value?.components.filter((c) => selectedComponents.value.includes(c.id)) ?? []
  if (!items.length) return []
  const fields: BatchField[] = [
    { id: 'title', label: t('rowEditor.name'), kind: 'text', required: true },
    { id: 'icon', label: t('rowEditor.icon'), kind: 'icon' },
  ]
  const kind = valueKind(items[0]!.kind)
  if (kind && items.every((c) => valueKind(c.kind) === kind)) {
    const selects = items.filter((c) => c.kind === 'select')
    if (selects.length) {
      const options = (selects[0]!.options ?? []).filter((value) =>
        selects.every((c) => c.options?.includes(value)),
      )
      if (options.length)
        fields.push({
          id: 'initial',
          label: t('panels.initial_value'),
          kind: 'select',
          options: options.map((value) => ({ value, label: value })),
        })
    } else fields.push({ id: 'initial', label: t('panels.initial_value'), kind })
  }
  return fields
})
function applyComponentBatch(field: string, value: string | number | boolean) {
  if (!draft.value || !componentBatchFields.value.some((item) => item.id === field)) return
  for (const c of draft.value.components) {
    if (!selectedComponents.value.includes(c.id)) continue
    if (field === 'title') c.title = String(value)
    if (field === 'icon') c.icon = String(value)
    if (field === 'initial') c.initial = value
  }
}
async function save() {
  if (busy.value) return
  if (!draft.value) return
  busy.value = true
  try {
    draft.value = await panelBackend.save(draft.value)
    selected.value = draft.value.id
    baseline.value = JSON.stringify(draft.value)
    failure.value = ''
    await refresh()
  } catch (e) {
    failure.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
async function discard() {
  draft.value = null
  failure.value = ''
  const item = source.value
  if (item) await select(item)
}
async function show() {
  if (!selected.value) return
  try {
    await panelBackend.show(selected.value)
  } catch (e) {
    failure.value = errorMessage(e)
  }
}
async function remove() {
  if (busy.value) return
  if (!draft.value?.id) return
  if (!confirmDelete.value) {
    confirmDelete.value = true
    return
  }
  busy.value = true
  try {
    await panelBackend.remove(draft.value.id, draft.value.revision)
    draft.value = null
    selected.value = ''
    confirmDelete.value = false
    await refresh()
  } catch (e) {
    failure.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
onMounted(refresh)
</script>

<style src="./SettingsView.css"></style>
<style scoped>
.panel-manager {
  --settings-accent: var(--ui-primary);
  --settings-section-bg: color-mix(in oklab, var(--settings-accent) 1.5%, var(--ui-surface));
  --settings-section-border: color-mix(in oklab, var(--settings-accent) 15%, var(--ui-border));
  --settings-row-hover-bg: color-mix(in oklab, var(--settings-accent) 2%, var(--ui-surface-hover));
}
</style>

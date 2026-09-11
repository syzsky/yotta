<template>
  <HudShell
    class="panel-window"
    :style="{
      background: `color-mix(in srgb, var(--ui-bg) ${display.backgroundOpacity}%, transparent)`,
      '--panel-surface-opacity': `${display.backgroundOpacity}%`,
    }"
    icon="i-tabler-layout-dashboard"
    :title="t('panels.title')"
    :close-title="t('panels.hide')"
    @close="hide"
  >
    <template #actions>
      <UButton
        size="sm"
        color="neutral"
        variant="ghost"
        :icon="tabsCollapsed ? 'i-tabler-chevron-down' : 'i-tabler-chevron-up'"
        :aria-label="t(tabsCollapsed ? 'panels.expand_tabs' : 'panels.collapse_tabs')"
        :title="t(tabsCollapsed ? 'panels.expand_tabs' : 'panels.collapse_tabs')"
        :aria-expanded="!tabsCollapsed"
        aria-controls="panel-tabs-bar"
        @click="tabsCollapsed = !tabsCollapsed"
      />
      <UPopover v-model:open="settingsOpen">
        <UButton
          size="sm"
          color="neutral"
          variant="ghost"
          icon="i-tabler-settings"
          :aria-label="t('panels.appearance')"
          @click="editDisplay"
        />
        <template #content>
          <form class="w-72 space-y-4 p-4" @submit.prevent="saveDisplay">
            <h2 class="text-sm font-semibold">{{ t('panels.appearance') }}</h2>
            <div class="grid grid-cols-2 gap-3">
              <UFormField :label="t('panels.width')"
                ><UInputNumber
                  v-model="displayDraft.width"
                  :min="240"
                  :max="3840"
                  :step="20"
                  class="w-full"
              /></UFormField>
              <UFormField :label="t('panels.height')"
                ><UInputNumber
                  v-model="displayDraft.height"
                  :min="160"
                  :max="2160"
                  :step="20"
                  class="w-full"
              /></UFormField>
              <UFormField :label="t('panels.columns')"
                ><USelect v-model="displayDraft.columns" :items="[1, 2, 3, 4]" class="w-full"
              /></UFormField>
              <UFormField :label="t('panels.content_size')"
                ><USelect v-model="displayDraft.size" :items="sizeItems" class="w-full"
              /></UFormField>
            </div>
            <UFormField
              :label="t('panels.background_opacity', { value: displayDraft.backgroundOpacity })"
              ><USlider v-model="displayDraft.backgroundOpacity" :min="0" :max="100" :step="5"
            /></UFormField>
            <UButton type="submit" :loading="savingDisplay">{{ t('panels.apply') }}</UButton>
          </form>
        </template>
      </UPopover>
      <UPopover v-model:open="clickThroughOpen">
        <UButton
          size="sm"
          color="neutral"
          variant="ghost"
          icon="i-tabler-pointer-off"
          :aria-label="t('panels.click_through')"
        />
        <template #content
          ><div class="w-64 space-y-3 p-4">
            <p class="text-sm">{{ t('panels.click_through_hint') }}</p>
            <UButton @click="enableClickThrough">{{ t('panels.click_through') }}</UButton>
          </div></template
        >
      </UPopover>
      <UButton
        size="xs"
        variant="ghost"
        :color="pinned ? 'primary' : 'neutral'"
        icon="i-tabler-pin"
        :aria-label="t('panels.pin')"
        :aria-pressed="pinned"
        @click="togglePin"
      />
    </template>
    <div class="flex h-full min-h-0 flex-col" data-testid="extension-panels">
      <div
        v-show="!tabsCollapsed"
        id="panel-tabs-bar"
        class="flex shrink-0 items-center gap-2 border-b border-default px-4 py-2"
      >
        <div
          class="flex min-w-0 flex-1 gap-1 overflow-x-auto"
          role="tablist"
          :aria-label="t('panels.tabs')"
        >
          <UButton
            v-for="(item, index) in sources"
            :id="`panel-tab-${index}`"
            :key="item.id"
            size="sm"
            variant="ghost"
            :color="selected === item.id ? 'primary' : 'neutral'"
            role="tab"
            :aria-selected="selected === item.id"
            :title="
              item.status === 'waiting' ? t('panels.waiting_input') : t(item.definition.titleKey)
            "
            aria-controls="panel-content"
            :tabindex="selected === item.id ? 0 : -1"
            @click="selected = item.id"
            @keydown="tabKey($event, index)"
            >{{ t(item.definition.titleKey)
            }}{{ item.id.startsWith('run:') ? ' · ' + item.generation.slice(-6) : '' }}
            <span
              v-if="item.status === 'waiting'"
              class="ml-1 size-1.5 rounded-full bg-warning"
              aria-hidden="true"
          /></UButton>
        </div>
        <UButton
          color="neutral"
          variant="ghost"
          size="xs"
          icon="i-tabler-refresh"
          :aria-label="t('panels.refresh')"
          @click="reload(true)"
        />
      </div>
      <div
        v-if="!sources.length && !listFailure"
        class="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center"
      >
        <UIcon name="i-tabler-layout-dashboard" class="size-8 text-muted" />
        <h2 class="text-base font-medium text-highlighted">{{ t('panels.empty') }}</h2>
        <p class="max-w-sm text-sm text-muted">{{ t('panels.empty_hint') }}</p>
      </div>
      <div v-else class="min-h-0 flex-1 overflow-y-auto p-3" :style="{ zoom: contentScale }">
        <UAlert
          v-if="listFailure || failure"
          color="error"
          variant="soft"
          class="mb-4"
          :title="t('panels.source_error')"
          :description="listFailure || failure"
        />
        <section
          v-if="source"
          id="panel-content"
          role="tabpanel"
          :aria-labelledby="`panel-tab-${selectedIndex}`"
          :aria-busy="loading"
          class="space-y-5"
        >
          <PanelRenderer
            :columns="display.columns"
            :key="source.id + (snapshot?.sessionId ?? '')"
            :components="source.definition.components"
            :waiting-buttons="source.managed ? (source.waitingComponents ?? []) : undefined"
            :snapshot="snapshot"
            :disabled="!snapshot || !!failure || snapshot.status === 'ended'"
            :busy="busy"
            :now="now"
            @action="dispatch"
          />
          <UAlert
            v-if="actionFailure"
            color="error"
            variant="soft"
            :title="t('panels.action_error')"
            :description="actionFailure"
          />
        </section>
      </div>
      <footer
        v-if="source"
        class="flex shrink-0 items-center justify-between gap-3 border-t border-default px-4 py-2 text-xs text-muted"
      >
        <span class="text-xs">{{ clickThrough ? t('panels.click_through') : statusLabel }}</span>
        <span v-if="source.waiting" class="text-primary">{{
          t('panels.workflow_waiting', { count: source.waiting })
        }}</span>
      </footer>
    </div>
  </HudShell>
</template>
<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Events } from '@wailsio/runtime'
import HudShell from '@/components/tools/HudShell.vue'
import PanelRenderer from '@/components/panels/PanelRenderer.vue'
import { panelBackend, type PanelSource } from '@/lib/panels'
import { loadPluginMessages } from '@/lib/plugins'
import { backend } from '@/lib/backend'
import { errorMessage } from '@/lib/invoke'
import { usePanelSession } from '@/composables/usePanelSession'
import { useSettingsStore } from '@/stores/settings'
const { t } = useI18n()
const settingsStore = useSettingsStore()
const displayDefaults = {
  width: 480,
  height: 520,
  columns: 2,
  size: 'small',
  backgroundOpacity: 100,
}
const display = computed(() => ({ ...displayDefaults, ...settingsStore.data?.ui.panelDisplay }))
const contentScales: Record<string, number> = { small: 0.85, medium: 1, large: 1.2 }
const contentScale = computed(() => contentScales[display.value.size] ?? 1)
const sizeItems = computed(() =>
  ['small', 'medium', 'large'].map((value) => ({ value, label: t(`panels.size_${value}`) })),
)
const displayDraft = ref({ ...displayDefaults })
const settingsOpen = ref(false)
const tabsCollapsed = ref(false)
const savingDisplay = ref(false)
const clickThrough = ref(false)
const clickThroughOpen = ref(false)
function editDisplay() {
  displayDraft.value = { ...display.value, width: window.innerWidth, height: window.innerHeight }
}
async function saveDisplay() {
  savingDisplay.value = true
  try {
    if (!(await settingsStore.patch({ ui: { panelDisplay: displayDraft.value } })))
      throw settingsStore.lastError
    await backend.tools.setPanelsSize(displayDraft.value.width, displayDraft.value.height)
    settingsOpen.value = false
  } catch (error) {
    listFailure.value = errorMessage(error)
  } finally {
    savingDisplay.value = false
  }
}
async function enableClickThrough() {
  try {
    await backend.tools.setPanelsClickThrough(true)
    clickThrough.value = true
    clickThroughOpen.value = false
  } catch (error) {
    listFailure.value = errorMessage(error)
  }
}
const sources = ref<PanelSource[]>([])
let requestedSelection = ''
const selected = ref('')
const visible = ref(!document.hidden)
const pinned = ref(true)
const listFailure = ref('')
const now = ref(Date.now())
const source = computed(() => sources.value.find((item) => item.id === selected.value))
const selectedIndex = computed(() => sources.value.findIndex((item) => item.id === selected.value))
const { snapshot, failure, actionFailure, busy, loading, refresh, dispatch } = usePanelSession(
  selected,
  visible,
)
const statusLabel = computed(() =>
  t(
    failure.value
      ? 'panels.unavailable'
      : source.value?.managed && snapshot.value
        ? 'panels.available'
        : source.value?.id.startsWith('run:') && snapshot.value?.status === 'waiting'
          ? 'panels.waiting_input'
          : snapshot.value
            ? `panels.${snapshot.value.status}`
            : 'panels.connecting',
  ),
)
let clock: ReturnType<typeof setInterval> | undefined
let off: (() => void) | undefined
let offSelection: (() => void) | undefined
let offClickThrough: (() => void) | undefined
let disposed = false
let refreshing = false
let catalogTimer: ReturnType<typeof setInterval> | undefined
async function reload(messages = false) {
  if (refreshing) return
  refreshing = true
  try {
    if (messages) await loadPluginMessages()
    const values = await panelBackend.list()
    if (disposed) return
    if (JSON.stringify(sources.value) !== JSON.stringify(values)) {
      const oldDefinitions = sources.value.map((item) => [item.id, item.generation])
      const newDefinitions = values.map((item) => [item.id, item.generation])
      if (!messages && JSON.stringify(oldDefinitions) !== JSON.stringify(newDefinitions)) {
        await loadPluginMessages()
        if (disposed) return
      }
      sources.value = values
    }
    const requested = await panelBackend.selected()
    if (requested && requested !== requestedSelection) {
      selected.value = requested
      requestedSelection = requested
    }
    if (!values.some((item) => item.id === selected.value)) selected.value = values[0]?.id ?? ''
    listFailure.value = ''
    void refresh()
  } catch (error) {
    if (!disposed) listFailure.value = errorMessage(error)
  } finally {
    refreshing = false
  }
}
async function hide() {
  visible.value = false
  try {
    await backend.tools.hidePanels()
  } catch (error) {
    visible.value = true
    listFailure.value = errorMessage(error)
  }
}
async function togglePin() {
  try {
    await backend.tools.setPanelsAlwaysOnTop(!pinned.value)
    pinned.value = !pinned.value
  } catch (error) {
    listFailure.value = errorMessage(error)
  }
}
function visibility() {
  visible.value = !document.hidden
  if (visible.value) void reload()
}
function tabKey(event: KeyboardEvent, index: number) {
  const total = sources.value.length
  if (!total || !['ArrowRight', 'ArrowLeft', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? total - 1
        : (index + (event.key === 'ArrowRight' ? 1 : -1) + total) % total
  selected.value = sources.value[next]!.id
  void nextTick(() => document.getElementById(`panel-tab-${next}`)?.focus())
}
onMounted(() => {
  document.documentElement.classList.add('panel-window-document')
  void settingsStore
    .load()
    .then(() => backend.tools.setPanelsSize(display.value.width, display.value.height))
    .catch((error) => {
      listFailure.value = errorMessage(error)
    })
  offClickThrough = Events.On('panels:click-through', (event) => {
    clickThrough.value = (Array.isArray(event.data) ? event.data[0] : event.data) === true
  })
  void reload(true)
  catalogTimer = setInterval(() => {
    if (visible.value) void reload()
  }, 1500)
  document.addEventListener('visibilitychange', visibility)
  offSelection = Events.On('panels:selected', (event) => {
    const id = Array.isArray(event.data) ? event.data[0] : event.data
    if (typeof id === 'string') {
      selected.value = id
      requestedSelection = id
      void reload()
    }
  })
  off = Events.On('panels:visibility', (event) => {
    const data = Array.isArray(event.data) ? event.data[0] : event.data
    visible.value = data === true
    if (visible.value) void reload()
  })
  clock = setInterval(() => {
    if (visible.value) now.value = Date.now()
  }, 1000)
})
onUnmounted(() => {
  document.documentElement.classList.remove('panel-window-document')
  offClickThrough?.()
  disposed = true
  clearInterval(clock)
  clearInterval(catalogTimer)
  off?.()
  offSelection?.()
  document.removeEventListener('visibilitychange', visibility)
})
</script>
<style>
html.panel-window-document,
html.panel-window-document body {
  background: transparent !important;
}
.panel-window
  [data-testid='extension-panels']
  :is(input, [role='combobox'], button, [role='switch']) {
  background-color: color-mix(
    in srgb,
    var(--ui-bg-elevated) var(--panel-surface-opacity),
    transparent
  );
}
</style>

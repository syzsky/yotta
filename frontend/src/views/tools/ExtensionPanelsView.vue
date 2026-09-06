<template>
  <HudShell
    icon="i-tabler-layout-dashboard"
    :title="t('panels.title')"
    :close-title="t('panels.hide')"
    @close="hide"
  >
    <template #actions>
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
      <div class="flex shrink-0 items-center gap-2 border-b border-default px-4 py-2">
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
      <div v-else class="min-h-0 flex-1 overflow-y-auto p-5">
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
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0 space-y-1">
              <h1 class="text-lg font-semibold text-highlighted">
                {{ t(source.definition.titleKey) }}
              </h1>
              <p v-if="source.definition.descriptionKey" class="text-sm text-muted">
                {{ t(source.definition.descriptionKey) }}
              </p>
            </div>
            <UBadge color="neutral" variant="soft">{{ statusLabel }}</UBadge>
          </div>
          <p v-if="source.waiting" class="text-sm text-primary">
            {{ t('panels.workflow_waiting', { count: source.waiting }) }}
          </p>
          <p v-if="source.updatedAt" class="text-xs text-muted">
            {{ t('panels.updated_at', { time: new Date(source.updatedAt).toLocaleTimeString() }) }}
            ·
            {{
              t(
                source.lastRunStatus === 'running'
                  ? 'panels.writer_running'
                  : 'panels.writer_finished',
              )
            }}
          </p>
          <p v-if="source.managed && !source.waiting" class="text-xs text-muted">
            {{ t('panels.no_workflow_waiting') }}
          </p>
          <PanelRenderer
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
        class="flex shrink-0 items-center justify-between gap-3 border-t border-default px-4 py-2 text-xs text-muted"
      >
        <span class="truncate">{{
          source?.id.startsWith('run:')
            ? t('panels.run_label', { id: source.generation.slice(-8) })
            : source?.managed
              ? t('panels.user_owned')
              : (source?.ownerName ?? t('panels.title'))
        }}</span
        ><span>{{ t('panels.hide_hint') }}</span>
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
const { t } = useI18n()
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
    if (JSON.stringify(sources.value) !== JSON.stringify(values)) sources.value = values
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
  disposed = true
  clearInterval(clock)
  clearInterval(catalogTimer)
  off?.()
  offSelection?.()
  document.removeEventListener('visibilitychange', visibility)
})
</script>

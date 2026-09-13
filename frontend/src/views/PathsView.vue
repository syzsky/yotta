<template>
  <HudShell
    class="path-editor"
    data-testid="paths-view"
    icon="i-tabler-map-route"
    :title="t('paths.editor_title')"
    :subtitle="t('paths.editor_subtitle')"
    :status="
      t(
        recording
          ? 'paths.recording_status'
          : recordingSession
            ? 'paths.paused_status'
            : saved
              ? 'paths.saved'
              : path.points.length
                ? 'paths.editing_status'
                : 'paths.ready',
      )
    "
    :status-active="recording"
    :close-title="t('common.close')"
    @close="requestClose"
  >
    <template #actions>
      <UButton
        size="xs"
        variant="ghost"
        :color="pinned ? 'primary' : 'neutral'"
        :icon="pinned ? 'i-tabler-pinned' : 'i-tabler-pin'"
        :aria-pressed="pinned"
        :aria-label="t(pinned ? 'paths.unpin' : 'paths.pin')"
        :title="t(pinned ? 'paths.unpin' : 'paths.pin')"
        :loading="pinBusy"
        @click="togglePin"
      />
      <UButton
        size="xs"
        color="neutral"
        variant="ghost"
        icon="i-tabler-file-plus"
        :label="t('paths.new')"
        :disabled="busy || recording"
        @click="newPath"
      />
      <UDropdownMenu :items="fileActions"
        ><UButton
          size="xs"
          color="neutral"
          variant="ghost"
          icon="i-tabler-dots"
          :aria-label="t('assets.library_actions')"
      /></UDropdownMenu>
    </template>
    <div class="path-layout" :inert="busy">
      <div class="path-workspace">
        <div class="path-toolbar">
          <UButton
            size="xs"
            variant="ghost"
            color="neutral"
            icon="i-tabler-arrows-maximize"
            :aria-label="t('screenPicker.fit')"
            @click="fitCanvas"
          />
          <UButton
            size="xs"
            variant="ghost"
            color="neutral"
            icon="i-tabler-minus"
            :disabled="zoom <= viewport.MIN_ZOOM"
            :aria-label="t('screenPicker.zoom_out')"
            @click="zoomCanvas(1 / 1.25)"
          />
          <span class="w-10 text-center text-[11px] tabular-nums text-muted"
            >{{ Math.round(zoom * 100) }}%</span
          >
          <UButton
            size="xs"
            variant="ghost"
            color="neutral"
            icon="i-tabler-plus"
            :disabled="zoom >= viewport.MAX_ZOOM"
            :aria-label="t('screenPicker.zoom_in')"
            @click="zoomCanvas(1.25)"
          />
          <USeparator orientation="vertical" class="mx-1 h-4" />
          <UButton
            size="xs"
            color="neutral"
            variant="ghost"
            icon="i-tabler-arrow-back-up"
            :aria-label="t('paths.undo')"
            :disabled="!history.length || recording"
            @click="undo"
          />
          <span class="flex-1" />
          <UButton
            size="xs"
            :icon="recording ? 'i-tabler-player-pause' : 'i-tabler-player-record'"
            :color="recording ? 'warning' : 'primary'"
            variant="soft"
            :label="
              t(
                recording
                  ? 'paths.pause'
                  : path.points.length
                    ? 'paths.record'
                    : 'paths.start_record',
              )
            "
            :disabled="!endpoint || sampling"
            @click="toggleRecording"
          />
          <UButton
            v-if="recordingSession"
            icon="i-tabler-player-stop"
            color="neutral"
            variant="soft"
            size="xs"
            :label="t('paths.finish_recording')"
            @click="finishRecording"
          />
          <UButton
            size="xs"
            icon="i-tabler-map-pin-plus"
            color="neutral"
            variant="soft"
            :label="t('paths.mark')"
            :disabled="!endpoint || sampling || recording"
            @click="mark(false)"
          />
        </div>
        <div
          ref="canvasElement"
          class="path-canvas"
          tabindex="0"
          :aria-label="t('paths.preview')"
          :class="{
            'cursor-grabbing': viewport.panning.value,
            'cursor-grab': viewport.spaceHeld.value,
          }"
          @wheel="wheelCanvas"
          @pointerdown="panCanvas"
          @pointermove="viewport.movePan($event.clientX, $event.clientY)"
          @pointerup="viewport.endPan"
          @pointercancel="releaseCanvas"
          @lostpointercapture="viewport.endPan"
          @keydown="canvasKeyDown"
          @keyup="viewport.onKeyUp"
          @blur="releaseCanvas"
          @contextmenu.prevent
        >
          <svg
            viewBox="0 0 600 400"
            class="absolute left-0 top-0"
            :style="{ ...viewport.transformStyle.value, width: '600px', height: '400px' }"
            role="img"
            :aria-label="t('paths.preview')"
          >
            <defs>
              <marker
                id="path-direction"
                viewBox="0 0 10 10"
                refX="9"
                refY="5"
                markerWidth="6"
                markerHeight="6"
                orient="auto-start-reverse"
              >
                <path d="M 0 0 L 10 5 L 0 10 z" fill="currentColor" class="text-primary" />
              </marker>
            </defs>
            <polyline
              v-if="projected.length > 1"
              :points="polyline"
              marker-end="url(#path-direction)"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              class="text-primary"
            />
            <g v-for="p in visibleMarkers" :key="p.id">
              <rect
                v-if="p.id === path.points[0]?.id"
                :x="p.x - 5"
                :y="p.y - 5"
                width="10"
                height="10"
                fill="currentColor"
                class="text-primary"
              />
              <circle v-else :cx="p.x" :cy="p.y" r="4" fill="currentColor" class="text-primary" />
              <circle
                v-if="p.id === selectedId"
                :cx="p.x"
                :cy="p.y"
                r="9"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                class="text-primary"
              />
              <text
                :x="p.x + 12"
                :y="p.y - 12"
                fill="currentColor"
                class="text-toned"
                font-size="14"
              >
                {{ path.points.findIndex((point) => point.id === p.id) + 1 }}
              </text>
            </g>
          </svg>
          <div v-if="!path.points.length" class="path-empty">
            <UIcon name="i-tabler-map-route" class="size-9 text-dimmed" />
            <p class="text-sm text-muted">{{ t('paths.canvas_empty') }}</p>
            <p class="max-w-64 text-center text-xs leading-5 text-dimmed">
              {{ t('paths.canvas_hint') }}
            </p>
          </div>
          <span v-if="path.points.length" class="absolute bottom-3 left-3 text-[11px] text-muted"
            >{{ path.reference.frame }} · {{ path.reference.unit }} ·
            {{ t('paths.direction', { count: path.points.length }) }}</span
          >
        </div>
        <section class="path-point-tray" :class="{ 'path-point-tray--empty': !path.points.length }">
          <div class="path-toolbar">
            <h2 class="text-xs font-medium text-toned">{{ t('paths.waypoints') }}</h2>
            <UBadge size="xs" color="neutral" variant="soft">{{ path.points.length }}</UBadge>
            <span class="flex-1" />
            <UButton
              size="xs"
              icon="i-tabler-plus"
              color="neutral"
              variant="ghost"
              :label="t('paths.add')"
              :disabled="recording"
              @click="add"
            />
            <UButton
              size="xs"
              icon="i-tabler-arrows-sort"
              color="neutral"
              variant="ghost"
              :aria-label="t('paths.reverse')"
              :disabled="recording || !path.points.length"
              @click="reverse"
            />
          </div>
          <div v-if="path.points.length" class="min-h-0 flex-1 overflow-auto">
            <div class="divide-y divide-default">
              <button
                v-for="(p, index) in pagePoints"
                :key="p.id"
                type="button"
                class="flex w-full items-center gap-3 px-3 py-2 text-left text-xs hover:bg-elevated focus-visible:outline-primary"
                :class="p.id === selectedId ? 'bg-elevated text-primary' : 'text-toned'"
                :aria-pressed="p.id === selectedId"
                @click="selectedId = p.id"
              >
                <span class="w-6 shrink-0 tabular-nums">{{ page * 50 + index + 1 }}</span>
                <span class="min-w-0 flex-1 truncate">{{ p.name || t('paths.unnamed') }}</span>
                <span class="shrink-0 tabular-nums text-muted"
                  >{{ p.x.toFixed(2) }}, {{ p.y.toFixed(2) }}</span
                >
              </button>
            </div>
            <div v-if="path.points.length > 50" class="mt-2 flex justify-end gap-2">
              <UButton
                color="neutral"
                variant="ghost"
                icon="i-tabler-chevron-left"
                :disabled="page === 0"
                :aria-label="t('paths.previous')"
                @click="page--"
              />
              <span class="self-center text-sm"
                >{{ page + 1 }} / {{ Math.ceil(path.points.length / 50) }}</span
              >
              <UButton
                color="neutral"
                variant="ghost"
                icon="i-tabler-chevron-right"
                :disabled="(page + 1) * 50 >= path.points.length"
                :aria-label="t('paths.next')"
                @click="page++"
              />
            </div>
          </div>
        </section>
      </div>
      <aside class="path-inspector">
        <div class="path-inspector-scroll">
          <UAlert v-if="problem" color="error" variant="soft" :description="problem" role="alert" />
          <section class="path-section">
            <h2 class="path-section-title">
              <UIcon name="i-tabler-current-location" class="size-3.5" />{{
                t('paths.position_source')
              }}
            </h2>
            <PositionSourcePicker
              v-model="endpoint"
              :disabled="recording"
              standalone
              @configure="backend.tools.openPathSettings('plugins').catch(report)"
            />
            <div class="flex items-center justify-between gap-2 text-xs text-muted">
              <span>{{ t('paths.mark_shortcut') }}</span>
              <UButton
                size="xs"
                variant="link"
                color="neutral"
                @click="backend.tools.openPathSettings('hotkeys').catch(report)"
                :label="markHotkey || t('paths.shortcut_unbound')"
                :aria-label="t('paths.hotkey_settings')"
              />
            </div>
            <details>
              <summary class="cursor-pointer text-xs text-muted">
                {{ t('paths.recording_settings') }}
              </summary>
              <UFormField
                class="mt-3"
                :label="`${t('paths.sampling_distance')} (${path.reference.unit})`"
                :description="t('paths.sampling_hint')"
              >
                <UInputNumber
                  v-model="samplingDistance"
                  size="sm"
                  class="w-full"
                  :min="0.01"
                  :disabled="recording"
                />
              </UFormField>
              <details class="border-t border-default pt-3">
                <summary class="cursor-pointer text-sm font-medium text-toned">
                  {{ t('paths.reference') }}
                </summary>
                <div class="mt-3 space-y-3">
                  <UFormField :label="t('paths.coordinate_kind')"
                    ><USelect
                      v-model="path.reference.kind"
                      :items="kindItems"
                      :disabled="recording || path.points.length > 0"
                      class="w-full"
                      @update:model-value="dirty = true"
                  /></UFormField>
                  <UFormField v-for="key in referenceTextKeys" :key="key" :label="t(`paths.${key}`)"
                    ><UInput
                      v-model="path.reference[key]"
                      :disabled="recording || path.points.length > 0"
                      class="w-full"
                      @update:model-value="dirty = true"
                  /></UFormField>
                  <UFormField :label="t('paths.axis_heading')"
                    ><UInputNumber
                      v-model="path.reference.axisHeading"
                      :min="0"
                      :max="359.999"
                      :disabled="recording || path.points.length > 0"
                      class="w-full"
                      @update:model-value="dirty = true"
                  /></UFormField>
                  <UFormField :label="t('paths.axis_sign')"
                    ><USelect
                      v-model="path.reference.axisSign"
                      :items="[-1, 1]"
                      :disabled="recording || path.points.length > 0"
                      class="w-full"
                      @update:model-value="dirty = true"
                  /></UFormField>
                  <p class="text-xs text-muted">{{ t('paths.reference_hint') }}</p>
                </div>
              </details>
            </details>
          </section>
          <section v-if="selected" class="path-section">
            <template v-if="selected">
              <h2 class="path-section-title">{{ t('paths.point_settings') }}</h2>
              <PathPointNameField
                :model-value="selected.name"
                :disabled="recording"
                @update:model-value="editPoint('name', $event)"
              />
              <div class="path-coordinate-fields">
                <UFormField label="X"
                  ><UInputNumber
                    :model-value="selected.x"
                    :disabled="recording"
                    class="w-full"
                    @update:model-value="editPoint('x', $event)"
                /></UFormField>
                <UFormField label="Y"
                  ><UInputNumber
                    :model-value="selected.y"
                    :disabled="recording"
                    class="w-full"
                    @update:model-value="editPoint('y', $event)"
                /></UFormField>
              </div>
              <UCheckbox
                :model-value="selected.z !== null"
                :label="t('paths.known_height')"
                :disabled="recording"
                @update:model-value="editPoint('z', $event ? 0 : null)"
              />
              <UInputNumber
                v-if="selected.z !== null"
                :model-value="selected.z"
                :aria-label="t('paths.height')"
                :disabled="recording"
                class="w-full"
                @update:model-value="editPoint('z', $event)"
              />
              <div class="flex flex-wrap gap-2">
                <UButton
                  icon="i-tabler-arrow-up"
                  color="neutral"
                  variant="soft"
                  :aria-label="t('paths.move_up')"
                  :disabled="recording || selectedIndex === 0"
                  @click="move(-1)"
                />
                <UButton
                  icon="i-tabler-arrow-down"
                  color="neutral"
                  variant="soft"
                  :aria-label="t('paths.move_down')"
                  :disabled="recording || selectedIndex === path.points.length - 1"
                  @click="move(1)"
                />
                <UButton
                  icon="i-tabler-trash"
                  color="error"
                  variant="ghost"
                  :aria-label="t('paths.remove_point')"
                  :disabled="recording"
                  @click="remove"
                />
                <UButton
                  color="neutral"
                  variant="ghost"
                  :label="t('paths.update_position')"
                  :disabled="recording || !endpoint || sampling"
                  @click="mark(true)"
                />
              </div>
            </template>
          </section>
          <section class="path-section">
            <h2 class="path-section-title">
              <UIcon name="i-tabler-file-description" class="size-3.5" />{{
                t('paths.resource_info')
              }}
            </h2>
            <UFormField :label="t('paths.name')" required
              ><UInput
                v-model="name"
                size="sm"
                class="w-full"
                maxlength="80"
                :placeholder="t('paths.name_placeholder')"
                @update:model-value="markDirty"
            /></UFormField>
            <UFormField :label="t('common.category')"
              ><UInputMenu
                v-model="category"
                :items="categoryOptions"
                create-item
                size="sm"
                class="w-full"
                :placeholder="t('recordingSave.category_placeholder')"
                @create="createCategory"
                @update:model-value="markDirty"
            /></UFormField>
            <UFormField :label="t('common.tags')"
              ><UInputMenu
                v-model="tags"
                :items="tagOptions"
                multiple
                create-item
                size="sm"
                class="w-full"
                :placeholder="t('recordingSave.tags_placeholder')"
                @create="createTag"
                @update:model-value="markDirty"
            /></UFormField>
          </section>
        </div>
        <footer class="path-footer">
          <UButton
            size="sm"
            color="neutral"
            variant="ghost"
            :label="t('common.cancel')"
            @click="requestClose"
          />
          <span class="flex-1" />
          <UButton
            size="sm"
            icon="i-tabler-check"
            :label="t(saved ? 'paths.saved' : 'paths.save_to_library')"
            :loading="busy"
            :disabled="recording || sampling || !name.trim() || !path.points.length"
            @click="save"
          />
        </footer>
      </aside>
    </div>
    <input
      ref="fileInput"
      type="file"
      accept=".json,application/json"
      class="hidden"
      @change="importPath"
    />
    <BaseModal v-model:open="deleteOpen" :title="t('paths.delete')">
      <p class="text-sm text-toned">{{ t('paths.delete_hint') }}</p>
      <template #footer
        ><UButton color="error" :label="t('paths.delete')" :loading="busy" @click="deletePath"
      /></template>
    </BaseModal>
    <BaseModal v-model:open="discardOpen" :title="t('paths.unsaved')">
      <p class="text-sm text-toned">{{ t('paths.unsaved_hint') }}</p>
      <template #footer
        ><UButton
          color="neutral"
          variant="soft"
          :label="t('paths.keep_editing')"
          @click="discardOpen = false" /><UButton
          color="warning"
          :label="t('paths.discard')"
          @click="discard"
      /></template>
    </BaseModal>
  </HudShell>
</template>
<script setup lang="ts">
import PathPointNameField from '@/app/paths/PathPointNameField.vue'
import PositionSourcePicker from '@/app/paths/PositionSourcePicker.vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { backend } from '@/lib/backend'
import { useAssetsStore } from '@/stores/assets'
import { errorMessage } from '@/lib/invoke'
import { usePickerViewport } from '@/composables/tools/usePickerViewport'
import HudShell from '@/components/tools/HudShell.vue'
import { useHotkeysStore } from '@/stores/hotkeys'
import BaseModal from '@/components/common/BaseModal.vue'
import {
  blankPath,
  parsePathDocument,
  previewPoints,
  sampledPoint,
  sampleIssue,
  sufficientlyDistant,
  type NavigationPath,
  type PathPoint,
  type PositionSample,
} from '@/app/paths/pathModel'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const assets = useAssetsStore()
const hotkeys = useHotkeysStore()
const pinned = ref(true)
const pinBusy = ref(false)
let offOpen: (() => void) | undefined
let offClose: (() => void) | undefined
async function togglePin() {
  pinBusy.value = true
  try {
    await backend.tools.setPathEditorAlwaysOnTop(!pinned.value)
    pinned.value = !pinned.value
  } catch (error) {
    report(error)
  } finally {
    pinBusy.value = false
  }
}
function requestClose() {
  if (busy.value) return
  guard(() => {
    finishRecording()
    void backend.tools.closePathEditor().catch(report)
  })
}
const markHotkey = computed(
  () => hotkeys.list.find((entry) => entry.key === 'paths.mark')?.hotkeyStr ?? '',
)
const canvasElement = ref<HTMLElement | null>(null)
const viewport = usePickerViewport(() => canvasElement.value)
const zoom = viewport.zoom
let autoFit = true
let resizeObserver: ResizeObserver | undefined
function fitCanvas() {
  autoFit = true
  viewport.fit(600, 400)
}
function zoomCanvas(factor: number) {
  const bounds = canvasElement.value?.getBoundingClientRect()
  if (!bounds) return
  autoFit = false
  viewport.zoomAt(bounds.left + bounds.width / 2, bounds.top + bounds.height / 2, factor)
}
function wheelCanvas(event: WheelEvent) {
  autoFit = false
  viewport.onWheel(event)
}
function panCanvas(event: PointerEvent) {
  canvasElement.value?.focus({ preventScroll: true })
  if (event.button !== 2 && !(event.button === 0 && viewport.spaceHeld.value)) return
  autoFit = false
  canvasElement.value?.setPointerCapture(event.pointerId)
  viewport.beginPan(event.clientX, event.clientY)
  event.preventDefault()
}
function canvasKeyDown(event: KeyboardEvent) {
  if (event.code === 'Space') {
    viewport.onKeyDown(event)
    event.preventDefault()
  }
}
function releaseCanvas() {
  viewport.endPan()
  viewport.spaceHeld.value = false
}
const category = ref(''),
  tags = ref<string[]>([]),
  description = ref('')
const categoryOptions = ref<string[]>([]),
  tagOptions = ref<string[]>([])
function createCategory(value: string) {
  category.value = value
  categoryOptions.value = [...new Set([...categoryOptions.value, value])]
  markDirty()
}
function createTag(value: string) {
  tags.value = [...new Set([...tags.value, value])]
  tagOptions.value = [...new Set([...tagOptions.value, value])]
  markDirty()
}
const fileActions = computed(() => [
  [
    {
      label: t('paths.import'),
      icon: 'i-tabler-file-import',
      disabled: recording.value || busy.value,
      onSelect: () => fileInput.value?.click(),
    },
    {
      label: t('paths.export'),
      icon: 'i-tabler-file-export',
      disabled: !path.value.points.length,
      onSelect: exportPath,
    },
  ],
  ...(guid.value
    ? [
        [
          {
            label: t('paths.delete'),
            icon: 'i-tabler-trash',
            color: 'error' as const,
            disabled: recording.value || busy.value,
            onSelect: () => {
              deleteOpen.value = true
            },
          },
        ],
      ]
    : []),
])
const path = ref<NavigationPath>(blankPath())
const guid = ref(''),
  name = ref(''),
  endpoint = ref(''),
  selectedId = ref(''),
  problem = ref('')
const busy = ref(false),
  dirty = ref(false),
  saved = ref(false),
  recordingSession = ref(false),
  recording = ref(false),
  sampling = ref(false),
  deleteOpen = ref(false),
  discardOpen = ref(false)
const history = ref<NavigationPath[]>([])
const page = ref(0),
  samplingDistance = ref(1),
  fileInput = ref<HTMLInputElement>()
const selectedIndex = computed(() => path.value.points.findIndex((p) => p.id === selectedId.value))
const selected = computed(() => path.value.points[selectedIndex.value])
const projected = computed(() => previewPoints(path.value.points))
const polyline = computed(() => projected.value.map((p) => `${p.x},${p.y}`).join(' '))
const visibleMarkers = computed(() =>
  projected.value.filter(
    (p, i) => i === 0 || i === projected.value.length - 1 || p.id === selectedId.value,
  ),
)
const pagePoints = computed(() => path.value.points.slice(page.value * 50, (page.value + 1) * 50))
const kindItems = computed(() => [
  { value: 'world', label: t('paths.world') },
  { value: 'local', label: t('paths.local') },
])
const referenceTextKeys = ['frame', 'unit', 'map', 'floor'] as const
let watching = ''
let offSample: (() => void) | undefined
let offMark: (() => void) | undefined
let previous: PositionSample | null = null
let generation = 0
let pending: (() => void) | null = null
function markDirty() {
  dirty.value = true
  saved.value = false
}
function checkpoint() {
  history.value.push(JSON.parse(JSON.stringify(path.value)))
  if (history.value.length > 30) history.value.shift()
  dirty.value = true
  saved.value = false
}
function stop() {
  recording.value = false
  generation++
  const session = watching
  watching = ''
  if (session) void backend.paths.stopWatching(session).catch(report)
}
function finishRecording() {
  stop()
  recordingSession.value = false
  previous = null
}
function reset() {
  finishRecording()
  path.value = blankPath()
  guid.value = ''
  name.value = ''
  category.value = ''
  tags.value = []
  description.value = ''
  fitCanvas()
  selectedId.value = ''
  history.value = []
  previous = null
  dirty.value = false
  saved.value = false
  page.value = 0
  problem.value = ''
}
function guard(action: () => void) {
  if (dirty.value) {
    pending = action
    discardOpen.value = true
  } else action()
}
function discard() {
  dirty.value = false
  discardOpen.value = false
  const action = pending
  pending = null
  action?.()
}
function newPath() {
  guard(reset)
}
function report(error: unknown) {
  problem.value = errorMessage(error)
}
function open(id: string) {
  guard(() => {
    void load(id)
  })
}
async function load(id: string) {
  busy.value = true
  try {
    const [value, metadata] = await Promise.all([backend.paths.get(id), backend.assets.get(id)])
    reset()
    path.value = value.path
    guid.value = value.guid
    name.value = value.name
    category.value = metadata.category ?? ''
    tags.value = metadata.tags ?? []
    description.value = metadata.description ?? ''
    selectedId.value = value.path.points[0]?.id ?? ''
  } catch (e) {
    report(e)
  } finally {
    busy.value = false
  }
}
async function save() {
  busy.value = true
  problem.value = ''
  try {
    const value = await backend.paths.save(guid.value, name.value, path.value)
    guid.value = value.guid
    await backend.assets.updateMeta(
      value.guid,
      name.value.trim(),
      description.value,
      category.value,
      tags.value,
    )
    dirty.value = false
    saved.value = true
    assets.invalidate()
  } catch (e) {
    report(e)
  } finally {
    busy.value = false
  }
}
async function deletePath() {
  busy.value = true
  try {
    await backend.assets.delete_(guid.value)
    deleteOpen.value = false
    reset()
    assets.invalidate()
  } catch (e) {
    report(e)
  } finally {
    busy.value = false
  }
}
function undo() {
  const value = history.value.pop()
  if (value) {
    path.value = value
    selectedId.value = value.points[0]?.id ?? ''
    page.value = 0
    dirty.value = true
    saved.value = false
  }
}
function add() {
  checkpoint()
  const point: PathPoint = {
    id: crypto.randomUUID(),
    name: '',
    x: selected.value?.x ?? 0,
    y: selected.value?.y ?? 0,
    z: selected.value?.z ?? null,
  }
  const index = selectedIndex.value < 0 ? path.value.points.length : selectedIndex.value + 1
  path.value.points.splice(index, 0, point)
  selectedId.value = point.id
  page.value = Math.floor(index / 50)
}
function editPoint(key: keyof PathPoint, value: string | number | null | undefined) {
  if (
    !selected.value ||
    value === undefined ||
    (typeof value === 'number' && !Number.isFinite(value))
  )
    return
  checkpoint()
  Object.assign(selected.value, { [key]: value })
}
function remove() {
  checkpoint()
  path.value.points.splice(selectedIndex.value, 1)
  selectedId.value = path.value.points[0]?.id ?? ''
  page.value = Math.min(page.value, Math.max(0, Math.ceil(path.value.points.length / 50) - 1))
}
function move(delta: number) {
  const index = selectedIndex.value
  if (index + delta < 0 || index + delta >= path.value.points.length) return
  checkpoint()
  const point = path.value.points.splice(index, 1)[0]!
  path.value.points.splice(index + delta, 0, point)
  page.value = Math.floor((index + delta) / 50)
}
function reverse() {
  checkpoint()
  path.value.points.reverse()
}
async function mark(update: boolean, automatic = false) {
  if (sampling.value || busy.value) return
  sampling.value = true
  const request = generation
  try {
    const sample = await backend.paths.sample(endpoint.value)
    if (request !== generation) return
    acceptSample(sample, update, automatic)
  } catch (e) {
    if (request === generation) {
      report(e)
      stop()
    }
  } finally {
    sampling.value = false
  }
}
function acceptSample(sample: PositionSample, update: boolean, automatic: boolean) {
  const issue = sampleIssue(path.value, previous, sample, Date.now())
  if (issue === 'replay' && automatic) return
  if (issue) {
    problem.value = t(`paths.sample_${issue}`)
    stop()
    return
  }
  previous = sample
  const point = sampledPoint(sample)
  const last = path.value.points.at(-1)
  if (automatic && last && !sufficientlyDistant(last, point, samplingDistance.value)) return
  if (!automatic) checkpoint()
  if (!path.value.points.length) path.value.reference = sample.reference
  if (update && selected.value)
    Object.assign(selected.value, { x: point.x, y: point.y, z: point.z })
  else {
    path.value.points.push(point)
    selectedId.value = point.id
    page.value = Math.floor((path.value.points.length - 1) / 50)
  }
  dirty.value = true
  saved.value = false
  problem.value = ''
}
async function toggleRecording() {
  if (recording.value) {
    stop()
    return
  }
  checkpoint()
  recordingSession.value = true
  recording.value = true
  generation++
  const session = crypto.randomUUID()
  watching = session
  try {
    await backend.paths.startWatching(session, endpoint.value)
  } catch (e) {
    if (watching === session) {
      report(e)
      stop()
    }
  }
}
function exportPath() {
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(path.value, null, 2)], { type: 'application/json' }),
  )
  const a = document.createElement('a')
  a.href = url
  a.download = `${name.value || 'path'}.json`
  a.click()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
async function importPath(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    if (file.size > 16 * 1024 * 1024) throw new Error()
    const request = generation
    const raw = parsePathDocument(await file.text())
    if (request !== generation) return
    checkpoint()
    path.value = raw
    selectedId.value = raw.points[0]?.id ?? ''
    page.value = 0
    previous = null
  } catch {
    problem.value = t('paths.import_invalid')
  } finally {
    input.value = ''
  }
}
onBeforeRouteLeave((to) => {
  if (!dirty.value && !recording.value) return true
  guard(() => {
    stop()
    void router.push(to.fullPath)
  })
  return false
})
onMounted(() => {
  offOpen = backend.tools.onPathEditorOpen((id) => {
    if (busy.value || (id && id === guid.value)) return
    if (id) open(id)
    else newPath()
  })
  offClose = backend.tools.onPathEditorClose(requestClose)
  resizeObserver = new ResizeObserver(() => {
    if (autoFit) viewport.fit(600, 400)
  })
  if (canvasElement.value) resizeObserver.observe(canvasElement.value)
  fitCanvas()
  if (typeof route.query.id === 'string' && route.query.id) void open(route.query.id)
  offSample = backend.paths.onSample((event) => {
    if (!recording.value || event.session !== watching) return
    if (event.problem) {
      report(event.problem)
      stop()
    } else if (event.sample) acceptSample(event.sample, false, true)
  })
  offMark = backend.paths.onMark(() => {
    if (endpoint.value && !recording.value && !busy.value) void mark(false)
  })
  void hotkeys.reload().catch(report)
  void assets
    .query({
      kind: 'path',
      page: 1,
      pageSize: 1,
      search: '',
      category: '',
      tags: [],
      sort: 'name_asc',
      thumbnailBudget: 0,
      recentGUIDs: [],
    })
    .then((result) => {
      categoryOptions.value = result.categories.map((item) => item.value)
      tagOptions.value = result.tags.map((item) => item.value)
    })
    .catch(report)
})
onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  releaseCanvas()
  stop()
  offOpen?.()
  offClose?.()
  offSample?.()
  offMark?.()
})
</script>

<style scoped>
/* Uses the screenshot editor's canvas/inspector structure and surface tokens. */
.path-editor {
  width: 100%;
  height: 100%;
}
.path-layout {
  display: flex;
  flex: 1;
  min-width: 0;
  min-height: 0;
}
.path-workspace {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.path-toolbar {
  display: flex;
  flex: none;
  min-height: 40px;
  align-items: center;
  gap: 2px;
  padding: 5px 8px;
  border-bottom: 1px solid var(--ui-border);
  background: color-mix(in oklab, var(--ui-bg-elevated) 28%, transparent);
}
.path-canvas {
  position: relative;
  flex: 1;
  min-height: 120px;
  overflow: hidden;
  background: var(--ui-bg);
}
.path-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 24px;
}
.path-point-tray {
  display: flex;
  flex-direction: column;
  flex: none;
  height: 190px;
  max-height: 30%;
  border-top: 1px solid var(--ui-border);
}
.path-point-tray--empty {
  height: 41px;
}
.path-inspector {
  container-type: inline-size;
  display: flex;
  flex-direction: column;
  width: clamp(280px, 26vw, 328px);
  min-height: 0;
  flex: none;
  border-left: 1px solid var(--ui-border);
  background: var(--ui-bg);
}
.path-inspector-scroll {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  min-height: 0;
  flex: 1;
  overflow-y: auto;
}
.path-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--ui-border);
  border-radius: 12px;
  background: color-mix(in oklab, var(--ui-bg-elevated) 24%, transparent);
}
.path-section-title {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 11px;
  line-height: 14px;
  font-weight: 650;
  color: var(--ui-text-muted);
}
.path-footer {
  display: flex;
  flex: none;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border-top: 1px solid var(--ui-border);
}
.path-coordinate-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
@container (max-width: 310px) {
  .path-coordinate-fields {
    grid-template-columns: minmax(0, 1fr);
  }
}
@media (max-width: 860px) {
  .path-inspector {
    width: 270px;
  }
  .path-toolbar {
    flex-wrap: wrap;
  }
}
@media (max-height: 560px) {
  .path-section {
    gap: 7px;
    padding: 9px;
  }
  .path-point-tray {
    height: 120px;
  }
  .path-point-tray--empty {
    height: 41px;
  }
}
</style>

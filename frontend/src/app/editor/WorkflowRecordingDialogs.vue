<template>
  <BaseModal
    v-model:open="recording.startOpen"
    :title="
      t(
        recording.mode === 'simple'
          ? 'workflow.recording.macro_title'
          : 'workflow.recording.precise_title',
      )
    "
    :icon="recording.mode === 'simple' ? 'i-tabler-list-details' : 'i-tabler-route-alt-left'"
    size="md"
  >
    <div class="space-y-3">
      <div class="flex items-start gap-3 rounded-lg border border-default bg-elevated/30 p-3">
        <UIcon
          :name="recording.mode === 'simple' ? 'i-tabler-list-details' : 'i-tabler-route-alt-left'"
          class="mt-0.5 size-5 shrink-0 text-primary"
        />
        <p class="text-xs leading-5 text-muted">
          {{
            t(
              recording.mode === 'simple'
                ? 'workflow.recording.macro_hint'
                : 'workflow.recording.precise_hint',
            )
          }}
        </p>
      </div>
      <UFormField :label="t('workflow.recording.target')" required>
        <AdaptiveSelect
          v-model="recording.targetSlot"
          :items="targets"
          value-key="value"
          label-key="label"
          :placeholder="t('assets.target_placeholder')"
        />
      </UFormField>
    </div>
    <p class="mt-3 text-xs leading-5 text-muted">{{ t('workflow.recording.start_hint') }}</p>
    <template #footer>
      <UButton color="neutral" variant="ghost" @click="recording.startOpen = false">{{
        t('common.cancel')
      }}</UButton>
      <UButton
        :disabled="!recording.targetSlot"
        :loading="recording.controlBusy"
        @click="emit('start')"
      >
        {{ t('workflow.recording.start') }}
      </UButton>
    </template>
  </BaseModal>

  <BaseModal
    :open="!!recording.pending"
    :title="t('recordingSave.title')"
    icon="i-tabler-list-check"
    size="3xl"
    :show-close="false"
    :dismissible="false"
  >
    <div v-if="recording.pending" class="space-y-4">
      <div v-if="recording.pending.mode === 'simple'" class="grid grid-cols-2 gap-3">
        <div class="rounded-lg border border-default bg-elevated/35 px-4 py-3">
          <p class="text-xs text-muted">{{ t('workflow.recording.result_mode') }}</p>
          <div class="mt-1 flex items-center gap-2">
            <UBadge
              :color="recording.pending.preview.mode === 'simple' ? 'primary' : 'warning'"
              variant="soft"
            >
              {{ t(`workflow.recording.mode_${recording.pending.preview.mode}`) }}
            </UBadge>
            <span class="text-xs text-toned">
              {{
                t('recordingSave.summary', {
                  duration: formatRecordingDuration(recording.pending.durationUs),
                  count: recording.pending.eventCount,
                })
              }}
            </span>
          </div>
        </div>
        <div class="rounded-lg border border-default bg-elevated/35 px-4 py-3 text-xs text-muted">
          {{
            t('workflow.recording.action_summary', {
              keys: recording.pending.preview.keyActions,
              clicks: recording.pending.preview.clickActions,
              moves: recording.pending.preview.pointerMoves + recording.pending.preview.rawDeltas,
              scrolls: recording.pending.preview.scrollActions,
            })
          }}
        </div>
      </div>
      <div
        v-if="recording.pending.mode === 'simple' && recording.pending.preview.steps.length"
        class="max-h-48 space-y-1 overflow-y-auto rounded-lg border border-default bg-sunken p-2"
      >
        <div
          v-for="(step, index) in recording.pending.preview.steps"
          :key="`${step.atUs}:${index}`"
          class="flex items-center gap-2 rounded-md px-2 py-1.5 text-xs text-muted"
        >
          <span class="w-12 shrink-0 font-mono text-[10px] text-dimmed"
            >{{ (step.atUs / 1_000_000).toFixed(2) }}s</span
          >
          <UIcon
            :name="
              step.kind.startsWith('key-')
                ? 'i-tabler-keyboard'
                : step.kind === 'sleep'
                  ? 'i-tabler-clock-pause'
                  : 'i-tabler-pointer'
            "
            class="size-4 shrink-0 text-primary"
          />
          <span class="truncate text-toned">
            {{
              step.kind.startsWith('key-')
                ? `${step.kind === 'key-down' ? '↓' : '↑'} ${step.key}`
                : step.kind === 'sleep'
                  ? `${Math.round(step.durationUs / 1000)} ms`
                  : `${step.button ?? step.kind} · ${Math.round((step.point?.x ?? 0) * 100)}%, ${Math.round((step.point?.y ?? 0) * 100)}%`
            }}
          </span>
        </div>
      </div>
      <PreciseRecordingWorkbench
        v-if="recording.pending.mode === 'precise'"
        :preview="recording.pending.preview"
        :environment="recording.pending.environment"
        :duration-us="recording.pending.durationUs"
        :trim-start-us="recording.trimStartUs"
        :trim-end-us="recording.trimEndUs"
        :pending-id="recording.pending.pendingID"
        editable-trim
        @update:trim-start-us="recording.trimStartUs = $event"
        @update:trim-end-us="recording.trimEndUs = $event"
      />
      <MacroActionEditor
        v-if="recording.pending.mode === 'simple' && recording.document"
        v-model="recording.document"
        @validity="recording.actionsValid = $event"
      />
      <p
        v-else-if="recording.pending.mode === 'simple'"
        class="rounded-lg border border-default bg-sunken px-3 py-2 text-xs text-muted"
      >
        {{ t('recordingEditor.editing_unavailable') }}
      </p>
      <RecordingMetadataFields
        v-model:name="recording.draft.name"
        v-model:description="recording.draft.description"
        v-model:category="recording.draft.category"
        v-model:tags="recording.draft.tags"
        :categories="recording.facetCategories"
        :tag-suggestions="recording.facetTags"
      />
    </div>
    <template #footer>
      <UButton
        color="error"
        variant="ghost"
        :disabled="recording.saveBusy"
        @click="emit('discard')"
      >
        {{ t('recordingSave.discard') }}
      </UButton>
      <UButton
        :loading="recording.saveBusy"
        :disabled="
          !recording.draft.name.trim() ||
          (recording.pending?.mode === 'simple' && !recording.actionsValid)
        "
        @click="emit('finalize')"
      >
        {{ t('assets.recording.save_to_library') }}
      </UButton>
    </template>
  </BaseModal>

  <BaseModal
    :open="!!macroEditing"
    :title="macroEditing?.label ?? t('macroEditor.title')"
    icon="i-tabler-list-details"
    size="5xl"
    tall
    @update:open="(open) => !open && (macroEditing = null)"
  >
    <div v-if="macroEditing" class="flex h-full min-h-0 flex-col gap-3">
      <div class="shrink-0 space-y-3 rounded-lg border border-default bg-elevated/20 p-3">
        <RecordingMetadataFields
          v-model:name="macroEditing.label"
          v-model:description="macroEditing.description"
          v-model:category="macroEditing.category"
          v-model:tags="macroEditing.tags"
          :categories="categories"
          :tag-suggestions="tags"
        />
      </div>
      <div
        class="flex shrink-0 items-center gap-3 rounded-lg border border-default bg-elevated/25 px-3 py-2 text-xs text-muted"
      >
        <span>{{ t('assets.macros.base_resolution') }}</span>
        <strong class="font-mono text-toned"
          >{{ macroEditing.document.baseResolution[0] }}×{{
            macroEditing.document.baseResolution[1]
          }}</strong
        >
        <span class="ml-auto truncate font-mono text-[10px] text-dimmed">{{
          macroEditing.id
        }}</span>
      </div>
      <MacroActionEditor
        v-model="macroEditing.document"
        class="min-h-0 flex-1"
        @validity="macroEditValid = $event"
      />
    </div>
    <template #footer>
      <UButton color="neutral" variant="ghost" @click="macroEditing = null">{{
        t('common.cancel')
      }}</UButton>
      <UButton
        icon="i-tabler-device-floppy"
        :loading="macroEditBusy"
        :disabled="!macroEditValid || !macroEditing?.label.trim()"
        @click="emit('save-macro')"
      >
        {{ t('common.save') }}
      </UButton>
    </template>
  </BaseModal>

  <BaseModal
    :open="!!workflowMacroEditing"
    :title="workflowMacroEditing?.resource.name ?? t('macroEditor.title')"
    icon="i-tabler-list-details"
    size="5xl"
    tall
    @update:open="(open) => !open && (workflowMacroEditing = null)"
  >
    <div v-if="workflowMacroEditing" class="flex h-full min-h-0 flex-col gap-3">
      <div class="shrink-0 space-y-3 rounded-lg border border-default bg-elevated/20 p-3">
        <RecordingMetadataFields
          v-model:name="workflowMacroEditing.resource.name"
          v-model:description="workflowMacroEditing.resource.description"
          v-model:category="workflowMacroEditing.resource.category"
          v-model:tags="workflowMacroEditing.resource.tags"
          :categories="categories"
          :tag-suggestions="tags"
        />
      </div>
      <div
        class="flex shrink-0 items-center gap-3 rounded-lg border border-default bg-elevated/25 px-3 py-2 text-xs text-muted"
      >
        <span>{{ t('assets.macros.base_resolution') }}</span>
        <strong class="font-mono text-toned"
          >{{ workflowMacroEditing.document.baseResolution[0] }}×{{
            workflowMacroEditing.document.baseResolution[1]
          }}</strong
        >
        <span class="ml-auto truncate font-mono text-[10px] text-dimmed">{{
          workflowMacroEditing.resource.id
        }}</span>
      </div>
      <MacroActionEditor
        v-model="workflowMacroEditing.document"
        class="min-h-0 flex-1"
        @validity="workflowMacroEditValid = $event"
      />
    </div>
    <template #footer>
      <UButton color="neutral" variant="ghost" @click="workflowMacroEditing = null">{{
        t('common.cancel')
      }}</UButton>
      <UButton
        icon="i-tabler-device-floppy"
        :loading="workflowResourceEditBusy"
        :disabled="!workflowMacroEditValid || !workflowMacroEditing?.resource.name.trim()"
        @click="emit('save-workflow-macro')"
      >
        {{ t('common.save') }}
      </UButton>
    </template>
  </BaseModal>

  <BaseModal
    :open="!!workflowClipEditing"
    :title="workflowClipEditing?.resource.name ?? t('preciseWorkbench.title')"
    icon="i-tabler-route-alt-left"
    size="5xl"
    tall
    @update:open="(open) => !open && (workflowClipEditing = null)"
  >
    <PreciseRecordingWorkbench
      v-if="workflowClipEditing && workflowClipPreview"
      :preview="workflowClipPreview"
      :environment="{
        baseResolution: workflowClipEditing.content.baseResolution,
        mouseMode: workflowClipEditing.content.mouseMode,
        mouseCounts360: workflowClipEditing.content.mouseCounts360,
      }"
      :duration-us="workflowClipEditing.content.durationUs"
      :trim-start-us="workflowClipTrimStartUs"
      :trim-end-us="workflowClipTrimEndUs"
      :workflow-resource="workflowClipEditing.resource"
      editable-trim
      @update:trim-start-us="workflowClipTrimStartUs = $event"
      @update:trim-end-us="workflowClipTrimEndUs = $event"
    />
    <template #footer>
      <UButton color="neutral" variant="ghost" @click="workflowClipEditing = null">{{
        t('common.cancel')
      }}</UButton>
      <UButton
        icon="i-tabler-cut"
        :loading="workflowResourceEditBusy"
        :disabled="!workflowClipTrimChanged"
        @click="emit('save-workflow-clip')"
      >
        {{ t('common.save') }}
      </UButton>
    </template>
  </BaseModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { MacroAsset, MacroDocument, WorkflowResourceContent } from '@/lib/backend'
import type { WorkflowResource } from '../../../../contracts/workflow/current/workflow-source'
import type { RecordingPreview } from '@/stores/recording'
import type { EditorRecordingState, EditorRecordingTarget } from './EditorRecordingController'
import { formatRecordingDuration } from './EditorRecordingController'
import BaseModal from '@/components/common/BaseModal.vue'
import AdaptiveSelect from '@/components/common/AdaptiveSelect.vue'
import MacroActionEditor from '@/components/recording/MacroActionEditor.vue'
import PreciseRecordingWorkbench from '@/components/recording/PreciseRecordingWorkbench.vue'
import RecordingMetadataFields from '@/components/recording/RecordingMetadataFields.vue'

defineProps<{
  targets: EditorRecordingTarget[]
  categories: string[]
  tags: string[]
  macroEditBusy: boolean
  workflowResourceEditBusy: boolean
  workflowClipPreview: RecordingPreview | null
  workflowClipTrimChanged: boolean
}>()

const recording = defineModel<EditorRecordingState>('recording', { required: true })
const macroEditing = defineModel<MacroAsset | null>('macroEditing', { required: true })
const macroEditValid = defineModel<boolean>('macroEditValid', { required: true })
const workflowMacroEditing = defineModel<{
  resource: WorkflowResource
  document: MacroDocument
} | null>('workflowMacroEditing', { required: true })
const workflowMacroEditValid = defineModel<boolean>('workflowMacroEditValid', { required: true })
const workflowClipEditing = defineModel<{
  resource: WorkflowResource
  content: NonNullable<WorkflowResourceContent['inputClip']>
} | null>('workflowClipEditing', { required: true })
const workflowClipTrimStartUs = defineModel<number>('workflowClipTrimStartUs', { required: true })
const workflowClipTrimEndUs = defineModel<number>('workflowClipTrimEndUs', { required: true })

const emit = defineEmits<{
  start: []
  discard: []
  finalize: []
  'save-macro': []
  'save-workflow-macro': []
  'save-workflow-clip': []
}>()
const { t } = useI18n()
</script>

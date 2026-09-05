<template>
  <WorkflowQuickAddMenu
    v-model:open="quickAddOpen"
    :items="quickAddIntent === 'insert-edge' ? insertableQuickAddItems : quickAddItems"
    :anchor="quickAddAnchor"
    @choose="emit('choose-quick-add', $event)"
  />

  <BaseModal
    v-model:open="graphDialogOpen"
    :title="graphDialogMode === 'create' ? t('workflow.graphs.new') : t('workflow.graphs.rename')"
    icon="i-tabler-folders"
    size="md"
  >
    <UFormField :label="t('common.name')" required>
      <UInput
        v-model="graphName"
        data-testid="workflow-graph-name"
        autofocus
        maxlength="256"
        @keydown.enter="emit('commit-graph')"
      />
    </UFormField>
    <template #footer>
      <UButton
        data-testid="workflow-graph-cancel"
        color="neutral"
        variant="ghost"
        :label="t('common.cancel')"
        @click="graphDialogOpen = false"
      />
      <UButton
        data-testid="workflow-graph-confirm"
        :disabled="!graphName.trim()"
        :label="t('common.confirm')"
        @click="emit('commit-graph')"
      />
    </template>
  </BaseModal>

  <BaseModal
    :open="Boolean(pendingConversion)"
    :title="t('workflow.connection.conversion_title')"
    icon="i-tabler-arrows-transfer-down"
    size="md"
    @update:open="(open) => !open && emit('cancel-conversion')"
  >
    <p class="text-xs leading-5 text-muted">
      {{
        t('workflow.connection.conversion_hint', {
          source: pendingConversion?.sourceType,
          target: pendingConversion?.targetType,
        })
      }}
    </p>
    <div class="mt-3 space-y-2">
      <button
        v-for="candidate in pendingConversion?.candidates ?? []"
        :key="candidate.nodeTypeId"
        type="button"
        data-testid="workflow-conversion-candidate"
        class="flex w-full items-center gap-3 rounded-lg border border-default px-3 py-3 text-left hover:border-primary/50 hover:bg-elevated focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        @click="emit('apply-conversion', candidate)"
      >
        <UIcon name="i-tabler-arrows-transfer-down" class="size-4 shrink-0 text-primary" />
        <span class="min-w-0 flex-1">
          <span class="block truncate text-xs font-semibold text-highlighted">{{
            conversionTitle(candidate)
          }}</span>
          <span class="mt-0.5 block text-[10px] text-muted">{{
            t('workflow.connection.conversion_cost', { cost: candidate.cost })
          }}</span>
        </span>
        <UBadge color="warning" variant="soft" size="sm">{{
          t(`workflow.connection.conversion_${candidate.kind}`)
        }}</UBadge>
      </button>
    </div>
    <template #footer
      ><UButton
        color="neutral"
        variant="ghost"
        :label="t('common.cancel')"
        @click="emit('cancel-conversion')"
    /></template>
  </BaseModal>

  <BaseModal
    :open="Boolean(pendingStatePromotion)"
    :title="t('workflow.state_panel.promote_title')"
    icon="i-tabler-database-plus"
    size="md"
    @update:open="(open) => !open && emit('cancel-state-promotion')"
  >
    <p class="text-xs leading-5 text-muted">
      {{ t('workflow.state_panel.promote_hint', { type: pendingStatePromotion?.typeLabel }) }}
    </p>
    <UFormField class="mt-3" :label="t('workflow.inspector.state_name_placeholder')" required>
      <UInput
        v-model="statePromotionName"
        data-testid="workflow-state-promotion-name"
        autofocus
        maxlength="128"
        @keydown.enter.prevent="emit('commit-state-promotion')"
      />
    </UFormField>
    <p v-if="statePromotionError" class="mt-2 text-[11px] text-error">{{ statePromotionError }}</p>
    <template #footer>
      <UButton
        color="neutral"
        variant="ghost"
        :label="t('common.cancel')"
        @click="emit('cancel-state-promotion')"
      />
      <UButton
        data-testid="workflow-state-promotion-confirm"
        :disabled="Boolean(statePromotionError)"
        :label="t('workflow.state_panel.promote_action')"
        @click="emit('commit-state-promotion')"
      />
    </template>
  </BaseModal>

  <BaseModal
    v-model:open="nodeSearchOpen"
    :title="t('workflow.node_search.title')"
    icon="i-tabler-search"
    size="lg"
  >
    <UInput
      v-model="nodeSearchQuery"
      data-testid="workflow-node-search-input"
      icon="i-tabler-search"
      autofocus
      :placeholder="t('workflow.node_search.placeholder')"
      @keydown.enter.prevent="emit('select-first-search-result')"
    />
    <div class="mt-3 max-h-96 space-y-1 overflow-y-auto">
      <button
        v-for="result in nodeSearchResults"
        :key="`${result.graphId}:${result.nodeId}`"
        type="button"
        class="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left hover:bg-elevated focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary"
        @click="emit('select-search-result', result)"
      >
        <UIcon :name="`i-tabler-${result.icon || 'box'}`" class="size-4 text-primary" />
        <span class="min-w-0 flex-1"
          ><span class="block truncate text-xs font-medium text-highlighted">{{
            result.label
          }}</span
          ><span class="block truncate font-mono text-[10px] text-dimmed">{{
            result.nodeId
          }}</span></span
        >
        <span class="shrink-0 text-[10px] text-muted">{{ result.graphId }}</span>
      </button>
      <div v-if="!nodeSearchResults.length" class="px-3 py-8 text-center text-xs text-muted">
        {{
          nodeSearchQuery.trim()
            ? t('workflow.node_search.no_results')
            : t('workflow.node_search.empty')
        }}
      </div>
    </div>
    <template #footer>
      <span class="mr-auto text-[11px] text-muted">{{
        t('workflow.node_search.result_count', { n: nodeSearchResults.length })
      }}</span>
      <UButton color="neutral" variant="ghost" @click="nodeSearchOpen = false">{{
        t('common.close')
      }}</UButton>
    </template>
  </BaseModal>

  <BaseModal
    v-model:open="templateCaptureOpen"
    :title="
      templateCaptureIntent?.mode === 'replace'
        ? t('workflow.resources.recapture')
        : templateCaptureIntent?.mode === 'append'
          ? t('workflow.resources.create_version')
          : t('assets.templates.capture')
    "
    icon="i-tabler-camera-plus"
    size="md"
  >
    <div class="space-y-3">
      <p class="text-xs leading-5 text-muted">{{ t('workflow.resources.capture_hint') }}</p>
      <UFormField :label="t('workflow.recording.target')" required>
        <AdaptiveSelect
          v-model="captureTargetSlot"
          :items="recordingTargetItems"
          value-key="value"
          label-key="label"
          :placeholder="t('assets.target_placeholder')"
        />
      </UFormField>
    </div>
    <template #footer>
      <UButton color="neutral" variant="ghost" @click="templateCaptureOpen = false">{{
        t('common.cancel')
      }}</UButton>
      <UButton
        icon="i-tabler-camera-plus"
        :disabled="!captureTargetSlot"
        :loading="templateCaptureBusy"
        @click="emit('capture-template')"
      >
        {{
          templateCaptureIntent?.mode === 'replace'
            ? t('workflow.resources.recapture')
            : templateCaptureIntent?.mode === 'append'
              ? t('workflow.resources.create_version')
              : t('assets.templates.capture')
        }}
      </UButton>
    </template>
  </BaseModal>

  <WorkflowSnippetModal
    :open="snippetModalOpen"
    :snippet-id="snippetDraft?.id ?? ''"
    :node-type-id="snippetDraft?.payload.nodeRef.nodeTypeId ?? ''"
    :initial="snippetModalInitial"
    :existing="snippets"
    :busy="snippetSaveBusy"
    @update:open="snippetModalOpen = $event"
    @save="emit('save-snippet', $event)"
  />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { WorkflowSnippet, WorkflowSnippetSummary } from '@/lib/backend'
import type { WorkflowQuickAddItem } from './workflowQuickAdd'
import type { WorkflowNodeSearchResult } from './useWorkflowNodeSearch'
import type { PendingConversion, PendingStatePromotion } from './useWorkflowConnectionAuthoring'
import type { ConversionCandidatePlan } from './connectionCompatibility'
import type { WorkflowSnippetMetadata } from './useWorkflowSnippetAuthoring'
import BaseModal from '@/components/common/BaseModal.vue'
import AdaptiveSelect from '@/components/common/AdaptiveSelect.vue'
import WorkflowQuickAddMenu from './WorkflowQuickAddMenu.vue'
import WorkflowSnippetModal from './WorkflowSnippetModal.vue'

defineProps<{
  quickAddIntent: 'add' | 'insert-edge'
  quickAddItems: WorkflowQuickAddItem[]
  insertableQuickAddItems: WorkflowQuickAddItem[]
  quickAddAnchor: { x: number; y: number }
  graphDialogMode: 'create' | 'rename'
  pendingConversion: PendingConversion | null
  conversionTitle: (candidate: ConversionCandidatePlan) => string
  pendingStatePromotion: PendingStatePromotion | null
  statePromotionError: string
  nodeSearchResults: WorkflowNodeSearchResult[]
  templateCaptureIntent: { mode: 'replace' | 'append' } | null
  recordingTargetItems: Array<{ label: string; value: string }>
  templateCaptureBusy: boolean
  snippetDraft: WorkflowSnippet | null
  snippetModalInitial?: {
    name: string
    description?: string
    category?: string
    tags: string[]
    shortcut?: string
  }
  snippets: WorkflowSnippetSummary[]
  snippetSaveBusy: boolean
}>()

const quickAddOpen = defineModel<boolean>('quickAddOpen', { required: true })
const graphDialogOpen = defineModel<boolean>('graphDialogOpen', { required: true })
const graphName = defineModel<string>('graphName', { required: true })
const statePromotionName = defineModel<string>('statePromotionName', { required: true })
const nodeSearchOpen = defineModel<boolean>('nodeSearchOpen', { required: true })
const nodeSearchQuery = defineModel<string>('nodeSearchQuery', { required: true })
const templateCaptureOpen = defineModel<boolean>('templateCaptureOpen', { required: true })
const captureTargetSlot = defineModel<string>('captureTargetSlot', { required: true })
const snippetModalOpen = defineModel<boolean>('snippetModalOpen', { required: true })

const emit = defineEmits<{
  'choose-quick-add': [item: WorkflowQuickAddItem]
  'commit-graph': []
  'cancel-conversion': []
  'apply-conversion': [candidate: ConversionCandidatePlan]
  'cancel-state-promotion': []
  'commit-state-promotion': []
  'select-first-search-result': []
  'select-search-result': [result: WorkflowNodeSearchResult]
  'capture-template': []
  'save-snippet': [metadata: WorkflowSnippetMetadata]
}>()
const { t } = useI18n()
</script>

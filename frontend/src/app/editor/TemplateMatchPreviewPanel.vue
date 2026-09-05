<template>
  <section class="space-y-3 border-t border-default pt-3" data-testid="template-match-preview">
    <div class="flex items-center justify-between gap-2">
      <h3 class="text-xs font-semibold text-highlighted">
        {{ t('workflow.inspector.template_preview_title') }}
      </h3>
      <UButton
        size="xs"
        :color="enabled ? 'primary' : 'neutral'"
        variant="soft"
        :icon="enabled ? 'i-tabler-player-stop' : 'i-tabler-player-play'"
        :label="
          t(
            enabled
              ? 'workflow.inspector.template_preview_stop'
              : 'workflow.inspector.template_preview_start',
          )
        "
        :disabled="!request && !enabled"
        :aria-pressed="enabled"
        @click="enabled = !enabled"
      />
    </div>
    <USelect
      v-if="!targetSlot"
      v-model="selectedTarget"
      :items="targetItems"
      :placeholder="t('workflow.inspector.template_preview_target')"
      :aria-label="t('workflow.inspector.template_preview_target')"
      class="w-full"
    />
    <div class="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-xs">
      <span class="text-muted">{{ t('workflow.inspector.template_preview_score') }}</span>
      <span
        class="font-mono text-base font-semibold tabular-nums"
        :class="result?.matched ? 'text-success' : 'text-highlighted'"
        >{{ score }}</span
      >
      <span v-if="result" :class="result.matched ? 'text-success' : 'text-warning'">{{
        t(
          result.matched
            ? 'workflow.inspector.template_preview_matched'
            : 'workflow.inspector.template_preview_unmatched',
        )
      }}</span>
      <UIcon
        v-if="loading"
        name="i-tabler-loader-2"
        class="ml-auto size-3.5 animate-spin text-muted"
        :aria-label="t('common.loading')"
      />
    </div>
    <p v-if="request" class="text-xs text-muted">
      {{
        t('workflow.inspector.template_preview_threshold', {
          value: (request.threshold * 100).toFixed(2),
        })
      }}
    </p>
    <p v-if="failure" role="alert" class="break-words text-xs leading-5 text-warning">
      {{ failure }}
    </p>
    <p v-else class="text-xs leading-5 text-muted">
      {{
        t(
          !request
            ? 'workflow.inspector.template_preview_required'
            : 'workflow.inspector.template_preview_hint',
        )
      }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WorkflowResource } from '../../../../contracts/workflow/current/workflow-source'
import type { Node, NodeProjection } from './EditorSession'
import { templatePreviewRequest } from './templateMatchPreview'
import { useTemplateMatchPreview } from './useTemplateMatchPreview'
import { useSettingsStore } from '@/stores/settings'

const props = defineProps<{
  node: Node
  projection: NodeProjection
  resources?: WorkflowResource[]
  targetSlot: string
  connectedInputIds?: ReadonlySet<string>
}>()
const { t } = useI18n()
const settings = useSettingsStore()
const selectedTarget = ref('')
const targetItems = computed(() =>
  (settings.data?.automation.targets ?? []).map((target) => ({
    label: target.label,
    value: target.slot,
  })),
)
const request = computed(() =>
  templatePreviewRequest(
    props.node,
    props.projection,
    props.resources ?? [],
    props.targetSlot || selectedTarget.value,
    props.connectedInputIds,
  ),
)
const { enabled, loading, result, failure } = useTemplateMatchPreview(request)
const score = computed(() =>
  result.value && result.value.score >= 0 ? `${(result.value.score * 100).toFixed(2)}%` : '—',
)
onMounted(() => {
  if (!settings.loaded) void settings.load()
})
</script>

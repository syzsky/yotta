<template>
  <div data-testid="workflow-publish-version" class="space-y-3">
    <div class="flex flex-wrap items-center gap-3">
      <UInput
        :model-value="modelValue"
        data-testid="workflow-version-text"
        :aria-label="t('workflow.market.version')"
        :aria-invalid="invalid || undefined"
        :disabled="disabled"
        :color="invalid ? 'error' : 'neutral'"
        placeholder="1.0.0"
        maxlength="128"
        class="w-48"
        :ui="{ base: 'tabular-nums' }"
        @update:model-value="emit('update:modelValue', String($event))"
      >
        <template #leading><span class="text-sm text-muted">v</span></template>
      </UInput>
      <span v-if="previous" class="text-xs text-muted">{{
        t('workflow.market.latest_version', { version: previous })
      }}</span>
    </div>
    <div v-if="previous" class="flex flex-wrap gap-2">
      <UButton
        v-for="component in [2, 1, 0] as const"
        :key="component"
        size="xs"
        color="neutral"
        variant="outline"
        :disabled="disabled"
        @click="emit('update:modelValue', nextRelease(previous, component))"
      >
        {{ t(labels[component]) }} · {{ nextRelease(previous, component) }}
      </UButton>
    </div>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { nextRelease } from '@/app/workflow-library/releaseVersion'
defineProps<{ modelValue: string; previous?: string; disabled?: boolean; invalid?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const { t } = useI18n()
const labels = [
  'workflow.market.version_major',
  'workflow.market.version_minor',
  'workflow.market.version_patch',
]
</script>

<template>
  <div
    data-testid="workflow-publish-version"
    class="grid grid-cols-[1fr_auto_1fr_auto_1fr] items-start gap-2"
  >
    <template v-for="(label, index) in labels" :key="label">
      <span v-if="index" class="pt-2 text-lg text-muted" aria-hidden="true">.</span>
      <label class="grid min-w-0 gap-1.5">
        <input
          :value="parts[index] ?? ''"
          :data-testid="`workflow-version-${index}`"
          :aria-label="label"
          :aria-invalid="invalid || undefined"
          :disabled="disabled"
          type="text"
          inputmode="numeric"
          pattern="0|[1-9][0-9]*"
          maxlength="40"
          class="h-10 w-full rounded-md border border-default bg-default px-3 text-center text-sm tabular-nums text-highlighted outline-none focus:border-primary focus:ring-1 focus:ring-primary disabled:opacity-60"
          @input="update(index, ($event.target as HTMLInputElement).value)"
        />
        <span class="text-center text-xs text-muted">{{ label }}</span>
      </label>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ modelValue: string; disabled?: boolean; invalid?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const { t } = useI18n()
const labels = computed(() => [
  t('workflow.market.version_major'),
  t('workflow.market.version_minor'),
  t('workflow.market.version_patch'),
])
const parts = computed(() => props.modelValue.split('.'))
function update(index: number, value: string) {
  const values = [parts.value[0] ?? '', parts.value[1] ?? '', parts.value[2] ?? '']
  values[index] = value
  emit('update:modelValue', values.join('.'))
}
</script>

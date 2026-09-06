<template>
  <PointValueEditor
    v-if="adapter === 'point'"
    :model-value="modelValue"
    :target-slot="targetSlot"
    :compact="compact"
    @update:model-value="emit('update:model-value', $event)"
  />
  <RegionValueEditor
    v-else-if="adapter === 'region'"
    :model-value="modelValue"
    :target-slot="targetSlot"
    :compact="compact"
    @update:model-value="emit('update:model-value', $event)"
  />
  <ColorRangeValueEditor
    v-else-if="adapter === 'color-range'"
    :model-value="modelValue"
    :target-slot="targetSlot"
    :compact="compact"
    @update:model-value="emit('update:model-value', $event)"
  />
  <DurationValueEditor
    v-else-if="adapter === 'duration'"
    :model-value="modelValue"
    :compact="compact"
    @update:model-value="emit('update:model-value', $event)"
  />
  <KeyChordValueEditor
    v-else-if="adapter === 'key-chord'"
    :model-value="keyChordValue"
    @update:model-value="emit('update:model-value', $event)"
  />
  <USwitch
    v-else-if="adapter === 'toggle'"
    :model-value="Boolean(modelValue)"
    :size="compact ? 'xs' : 'sm'"
    @update:model-value="emit('update:model-value', $event)"
  />
  <UInputNumber
    v-else-if="adapter === 'number'"
    :model-value="numberValue"
    :min="numericConstraint(port.type.constraints.minimum)"
    :max="numericConstraint(port.type.constraints.maximum)"
    :step="port.type.control === 'integer' ? 1 : 0.01"
    :step-snapping="false"
    :format-options="{ maximumFractionDigits: port.type.control === 'integer' ? 0 : 20 }"
    :size="compact ? 'xs' : 'sm'"
    class="w-full"
    @update:model-value="emit('update:model-value', Number($event))"
  />
  <AdaptiveSelect
    v-else-if="adapter === 'select'"
    :model-value="selectValue"
    :items="port.type.constraints.enum.map((value) => ({ label: String(value), value }))"
    width-mode="fill"
    :size="compact ? 'xs' : 'sm'"
    class="w-full"
    @update:model-value="emit('update:model-value', $event)"
  />
  <UInput
    v-else-if="adapter === 'text'"
    :model-value="textValue"
    :size="compact ? 'xs' : 'sm'"
    class="w-full"
    @update:model-value="emit('update:model-value', $event)"
  />
  <UTextarea
    v-else-if="adapter === 'multiline-text'"
    :model-value="textValue"
    :rows="compact ? 3 : 8"
    :size="compact ? 'xs' : 'sm'"
    autoresize
    class="w-full text-sm leading-relaxed"
    @update:model-value="emit('update:model-value', $event)"
  />
  <template v-else>
    <UTextarea
      v-model="jsonDraft"
      :rows="compact ? 2 : 5"
      :size="compact ? 'xs' : 'sm'"
      :aria-invalid="Boolean(jsonError)"
      class="w-full font-mono text-xs"
      @blur="commitJSON"
    />
    <p v-if="jsonError" role="alert" class="mt-1 text-xs text-error">{{ jsonError }}</p>
  </template>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PortProjection } from '../../../../contracts/node/current/authoring-projection'
import type { ValueEditorAdapter } from './authoringSurface'
import AdaptiveSelect from '@/components/common/AdaptiveSelect.vue'

const PointValueEditor = defineAsyncComponent(() => import('./PointValueEditor.vue'))
const RegionValueEditor = defineAsyncComponent(() => import('./RegionValueEditor.vue'))
const ColorRangeValueEditor = defineAsyncComponent(() => import('./ColorRangeValueEditor.vue'))
const DurationValueEditor = defineAsyncComponent(() => import('./DurationValueEditor.vue'))
const KeyChordValueEditor = defineAsyncComponent(() => import('./KeyChordValueEditor.vue'))

const props = defineProps<{
  adapter: ValueEditorAdapter
  port: PortProjection
  modelValue: unknown
  targetSlot?: string
  compact?: boolean
}>()
const emit = defineEmits<{ 'update:model-value': [value: unknown] }>()
const { t } = useI18n()
const jsonDraft = ref('')
const jsonError = ref('')
const keyChordValue = computed(() =>
  Array.isArray(props.modelValue)
    ? props.modelValue.filter((value): value is string => typeof value === 'string')
    : [],
)
const numberValue = computed(() =>
  typeof props.modelValue === 'number' ? props.modelValue : undefined,
)
const textValue = computed(() => (typeof props.modelValue === 'string' ? props.modelValue : ''))
const selectValue = computed(() =>
  typeof props.modelValue === 'string' ||
  typeof props.modelValue === 'number' ||
  typeof props.modelValue === 'boolean'
    ? props.modelValue
    : null,
)
const jsonValue = computed(() =>
  props.modelValue === undefined
    ? ''
    : JSON.stringify(props.modelValue, null, props.compact ? 0 : 2),
)
watch(
  jsonValue,
  (value) => {
    jsonDraft.value = value
    jsonError.value = ''
  },
  { immediate: true },
)

function numericConstraint(value: unknown): number | undefined {
  return typeof value === 'number' ? value : undefined
}

function commitJSON(): void {
  try {
    const value: unknown = JSON.parse(jsonDraft.value)
    jsonError.value = ''
    emit('update:model-value', value)
  } catch {
    jsonError.value = t('workflow.inspector.invalid_json')
  }
}
</script>

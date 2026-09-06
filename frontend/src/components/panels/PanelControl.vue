<template>
  <div class="space-y-2">
    <label :for="controlId" class="flex items-center gap-2 text-xs text-muted"
      ><UIcon
        v-if="component.icon"
        :name="component.icon"
        class="size-4 shrink-0"
        aria-hidden="true"
      />{{ t(component.titleKey) }}</label
    >
    <USelect
      v-if="component.kind === 'select'"
      :id="controlId"
      class="w-full"
      :model-value="String(draft ?? '')"
      :items="options"
      :disabled="disabled"
      @update:model-value="change"
    />
    <USwitch
      v-else-if="component.kind === 'toggle'"
      :id="controlId"
      :model-value="Boolean(draft)"
      :disabled="disabled"
      @update:model-value="change"
    />
    <form v-else class="flex gap-2" @submit.prevent="change(draft)">
      <UInput
        :id="controlId"
        v-model="draft"
        class="min-w-0 flex-1"
        :disabled="disabled"
        @focus="editing = true"
        @blur="editing = false"
      />
      <UButton type="submit" color="neutral" variant="soft" :disabled="disabled">{{
        t('panels.apply')
      }}</UButton>
    </form>
  </div>
</template>
<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PanelComponent } from '@/lib/panels'
const props = defineProps<{
  component: PanelComponent
  value: string | number | boolean | undefined
  disabled: boolean
  pending: boolean
}>()
const emit = defineEmits<{ change: [value: unknown] }>()
const { t } = useI18n()
const controlId = useId()
const draft = ref<string | number | boolean>(props.value ?? '')
const editing = ref(false)
const options = computed(() =>
  (props.component.options ?? []).map((option) => ({
    label: t(option.labelKey),
    value: option.value,
  })),
)
watch(
  () => [props.value, props.pending],
  () => {
    if (!editing.value && !props.pending) draft.value = props.value ?? ''
  },
)
function change(value: unknown) {
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean')
    draft.value = value
  emit('change', value)
}
</script>

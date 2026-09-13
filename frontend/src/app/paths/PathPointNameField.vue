<template>
  <UFormField :label="t('paths.point_name')" :description="t('paths.point_marker_hint')">
    <UInput
      :model-value="modelValue"
      :aria-label="t('paths.point_name')"
      :placeholder="t('paths.point_marker_placeholder')"
      :disabled="disabled"
      class="w-full"
      @update:model-value="emit('update:modelValue', String($event))"
    />
  </UFormField>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ modelValue: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const { t } = useI18n()
</script>

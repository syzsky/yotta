<template>
  <UPopover v-model:open="open" :ui="{ content: 'w-[300px] p-2' }">
    <UButton
      size="xs"
      color="neutral"
      variant="ghost"
      :icon="modelValue || fallback || 'i-tabler-photo-plus'"
      :aria-label="t('rowEditor.pick_icon')"
      :title="t('rowEditor.pick_icon')"
    />
    <template #content
      ><IconPicker :model-value="modelValue" @update:model-value="choose" /><UButton
        v-if="modelValue"
        class="mt-2"
        size="xs"
        color="neutral"
        variant="ghost"
        :label="t('rowEditor.clear_icon')"
        @click="choose('')"
    /></template>
  </UPopover>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ensureWorkflowIcons } from '@/lib/workflowIcons'
import IconPicker from '@/components/common/IconPicker.vue'
const props = defineProps<{ modelValue?: string; fallback?: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()
const { t } = useI18n()
const open = ref(false)
watch(
  () => props.modelValue,
  (icon) => {
    if (icon) void ensureWorkflowIcons([icon]).catch(() => undefined)
  },
  { immediate: true },
)
function choose(icon: string) {
  emit('update:modelValue', icon)
  open.value = false
}
</script>

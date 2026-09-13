<template>
  <USelectMenu
    v-model="model"
    :aria-label="t('paths.variable_type')"
    :items="items"
    value-key="value"
    label-key="label"
    class="min-w-0 w-full"
    :search-input="{ placeholder: t('workflow.state_panel.search') }"
  >
    <template #leading
      ><span
        class="size-2 shrink-0 rounded-full"
        :style="{ backgroundColor: items.find((item) => item.value === model)?.color }"
        aria-hidden="true"
    /></template>
    <template #item-leading="{ item }"
      ><span
        class="size-2 shrink-0 rounded-full"
        :style="{ backgroundColor: item.color }"
        aria-hidden="true"
    /></template>
  </USelectMenu>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ items: Array<{ value: string; label: string; color: string }> }>()
const model = defineModel<string>({ required: true })
const { t } = useI18n()
</script>

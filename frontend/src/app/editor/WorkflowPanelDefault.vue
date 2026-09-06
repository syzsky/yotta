<template>
  <UPopover mode="click" :ui="{ content: 'w-80 p-3' }" @update:open="onOpen">
    <UButton
      data-testid="workflow-panel-default"
      icon="i-tabler-layout-dashboard"
      color="neutral"
      variant="ghost"
      size="xs"
      :aria-label="t('panels.workflow_default')"
      :title="t('panels.workflow_default') + (selectedLabel ? ' · ' + selectedLabel : '')"
    />
    <template #content>
      <div class="space-y-2" data-testid="workflow-panel-default-settings">
        <p class="text-xs font-medium text-highlighted">{{ t('panels.workflow_default') }}</p>
        <AdaptiveSelect
          :model-value="modelValue"
          :items="displayOptions"
          value-key="value"
          label-key="label"
          width-mode="fill"
          :placeholder="t('panels.select_panel')"
          :aria-label="t('panels.workflow_default')"
          @update:model-value="emit('update:modelValue', String($event ?? ''))"
        />
        <UButton
          v-if="modelValue"
          color="neutral"
          variant="soft"
          size="xs"
          icon="i-tabler-external-link"
          :label="t('panels.show')"
          @click="showSelected"
        />
        <div class="flex items-center justify-between gap-2">
          <UButton
            v-if="modelValue"
            color="neutral"
            variant="ghost"
            size="xs"
            icon="i-tabler-x"
            @click="emit('update:modelValue', '')"
            >{{ t('panels.clear_default') }}</UButton
          >
          <RouterLink to="/panels" class="ml-auto text-xs text-primary hover:underline">{{
            t('panels.manage')
          }}</RouterLink>
        </div>
        <p v-if="failure" role="alert" class="text-xs text-error">{{ failure }}</p>
      </div>
    </template>
  </UPopover>
</template>
<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { panelBackend } from '@/lib/panels'
import { errorMessage } from '@/lib/invoke'
import { usePanelCatalog } from '@/composables/usePanelCatalog'
const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [string] }>()
const { t, te } = useI18n()
const { items, failure, refresh } = usePanelCatalog()
const options = computed(() =>
  items.value.map((p) => ({
    value: p.id,
    label:
      (te(p.definition.titleKey) ? t(p.definition.titleKey) : p.definition.titleKey) +
      (items.value.filter((x) => x.definition.titleKey === p.definition.titleKey).length > 1
        ? ' · ' + p.id.slice(-6)
        : ''),
  })),
)
const displayOptions = computed(() =>
  props.modelValue && !options.value.some((o) => o.value === props.modelValue)
    ? [{ value: props.modelValue, label: t('panels.missing_selection') }, ...options.value]
    : options.value,
)
const selectedLabel = computed(
  () => displayOptions.value.find((item) => item.value === props.modelValue)?.label ?? '',
)
function onOpen(open: boolean) {
  if (open) void refresh()
}
async function showSelected() {
  try {
    await panelBackend.show(props.modelValue)
  } catch (error) {
    failure.value = errorMessage(error)
  }
}
onMounted(refresh)
</script>

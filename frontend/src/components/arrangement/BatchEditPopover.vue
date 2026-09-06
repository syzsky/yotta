<template>
  <UPopover v-model:open="open" :ui="{ content: 'w-80 p-3' }">
    <UButton
      size="xs"
      variant="soft"
      color="neutral"
      icon="i-tabler-edit"
      :disabled="!fields.length"
      :label="t('rowEditor.batch_edit')"
      :title="!fields.length ? t('rowEditor.no_common_fields') : undefined"
    />
    <template #content
      ><form data-testid="arrangement-batch-editor" class="space-y-3" @submit.prevent="apply">
        <UFormField :label="t('rowEditor.field')"
          ><AdaptiveSelect
            v-model="fieldId"
            :aria-label="t('rowEditor.field')"
            :items="fields.map((field) => ({ value: field.id, label: field.label }))"
            width-mode="fill"
        /></UFormField>
        <template v-if="field">
          <IconPicker
            v-if="field.kind === 'icon'"
            :model-value="String(value)"
            @update:model-value="value = $event"
          />
          <UFormField v-else :label="t('rowEditor.value')">
            <USwitch
              v-if="field.kind === 'boolean'"
              :model-value="Boolean(value)"
              @update:model-value="value = $event"
            />
            <AdaptiveSelect
              v-else-if="field.kind === 'select'"
              :model-value="String(value)"
              :items="field.options ?? []"
              width-mode="fill"
              @update:model-value="value = String($event)"
            />
            <UInput
              v-else-if="field.kind === 'number'"
              type="number"
              :aria-label="t('rowEditor.value')"
              class="w-full"
              :model-value="Number(value)"
              @update:model-value="value = Number($event)"
            />
            <UInput v-else v-model="value" class="w-full" :aria-label="t('rowEditor.value')" />
          </UFormField>
          <UButton
            v-if="field.kind === 'icon'"
            size="xs"
            variant="ghost"
            color="neutral"
            :label="t('rowEditor.clear_icon')"
            @click="value = ''"
          />
        </template>
        <UButton
          type="submit"
          size="sm"
          variant="soft"
          :disabled="!valid"
          :label="t('rowEditor.apply', { count })"
        /></form
    ></template>
  </UPopover>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AdaptiveSelect from '@/components/common/AdaptiveSelect.vue'
import IconPicker from '@/components/common/IconPicker.vue'
import type { BatchField } from './batchFields'
const props = defineProps<{ fields: BatchField[]; count: number }>()
const emit = defineEmits<{ apply: [field: string, value: string | number | boolean] }>()
const { t } = useI18n()
const open = ref(false),
  fieldId = ref(''),
  value = ref<string | number | boolean>('')
const field = computed(() => props.fields.find((field) => field.id === fieldId.value))
watch(
  () => props.fields.map((field) => field.id + ':' + field.kind).join(','),
  () => {
    if (!field.value) fieldId.value = props.fields[0]?.id ?? ''
  },
  { immediate: true },
)
watch(
  () => field.value,
  (field) => {
    value.value =
      field?.kind === 'number'
        ? 0
        : field?.kind === 'boolean'
          ? false
          : field?.kind === 'select'
            ? (field.options?.[0]?.value ?? '')
            : ''
  },
  { immediate: true },
)
const valid = computed(
  () =>
    Boolean(field.value) &&
    props.count > 0 &&
    (!field.value?.required || String(value.value).trim() !== '') &&
    (field.value?.kind !== 'number' || Number.isFinite(Number(value.value))),
)
function apply() {
  if (!field.value || !valid.value) return
  emit('apply', field.value.id, value.value)
  open.value = false
}
</script>

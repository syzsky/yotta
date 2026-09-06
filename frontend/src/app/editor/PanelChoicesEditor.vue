<template>
  <div class="space-y-2">
    <div v-for="(_, index) in draft" :key="index" class="flex items-center gap-1.5">
      <UInput
        v-model="draft[index]"
        class="min-w-0 flex-1"
        :maxlength="128"
        :aria-label="t('panels.option_label', { n: index + 1 })"
        @focus="editing = true"
        @blur="commit"
        @keydown.enter.prevent="commit"
      />
      <UButton
        icon="i-tabler-chevron-up"
        color="neutral"
        variant="ghost"
        size="xs"
        :disabled="index === 0"
        :aria-label="t('panels.option_up')"
        @click="move(index, -1)"
      />
      <UButton
        icon="i-tabler-trash"
        color="neutral"
        variant="ghost"
        size="xs"
        :disabled="draft.length <= 1"
        :aria-label="t('panels.option_remove', { n: index + 1 })"
        @click="remove(index)"
      />
    </div>
    <p v-if="invalid" class="text-xs text-error" role="status">{{ t('panels.options_invalid') }}</p>
    <UButton
      icon="i-tabler-plus"
      color="neutral"
      variant="soft"
      size="xs"
      :disabled="draft.length >= 128"
      @click="add"
      >{{ t('panels.option_add') }}</UButton
    >
  </div>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ modelValue: unknown }>()
const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()
const { t } = useI18n()
const draft = ref<string[]>([])
const invalid = ref(false)
const editing = ref(false)
watch(
  () => props.modelValue,
  (value) => {
    if (!editing.value && Array.isArray(value) && value.every((item) => typeof item === 'string'))
      draft.value = [...value]
  },
  { immediate: true },
)
function commit() {
  editing.value = false
  const values = draft.value.map((value) => value.trim())
  invalid.value =
    !values.length || values.some((value) => !value) || new Set(values).size !== values.length
  if (!invalid.value) emit('update:modelValue', values)
}
function add() {
  let n = draft.value.length + 1
  let value = t('panels.option_label', { n })
  while (draft.value.includes(value)) value = t('panels.option_label', { n: ++n })
  draft.value.push(value)
  commit()
}
function remove(index: number) {
  draft.value.splice(index, 1)
  commit()
}
function move(index: number, offset: number) {
  const value = draft.value.splice(index, 1)[0]!
  draft.value.splice(index + offset, 0, value)
  commit()
}
</script>

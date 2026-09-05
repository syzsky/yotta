<template>
  <div
    class="overflow-hidden rounded-md border border-default bg-default"
    :class="{ 'opacity-60': disabled }"
  >
    <div class="flex items-center justify-between gap-2 border-b border-default bg-muted px-2 py-1">
      <div class="flex gap-1" :aria-label="label">
        <UButton
          v-for="value in modes"
          :key="value"
          size="xs"
          color="neutral"
          :variant="mode === value ? 'soft' : 'ghost'"
          :aria-pressed="mode === value"
          @click="mode = value"
          >{{ t(`workflow.markdown.${value}`) }}</UButton
        >
      </div>
      <span class="text-xs text-muted">Markdown</span>
    </div>
    <UEditor
      v-if="mode === 'edit'"
      v-slot="{ editor }"
      v-model="value"
      content-type="markdown"
      :editable="!disabled"
      :image="false"
      :mention="false"
      :placeholder="placeholder"
      :editor-props="{
        attributes: { 'aria-label': label, role: 'textbox', 'aria-multiline': 'true' },
      }"
      :ui="{ base: 'min-h-32 max-h-64 overflow-y-auto px-3 py-3 text-sm' }"
    >
      <UEditorToolbar
        :editor="editor"
        :items="toolbar"
        size="xs"
        class="overflow-x-auto border-b border-default p-1"
      />
    </UEditor>
    <UTextarea
      v-else-if="mode === 'source'"
      v-model="value"
      :aria-label="label"
      :disabled="disabled"
      :rows="7"
      :placeholder="placeholder"
      variant="none"
      class="w-full font-mono text-sm"
      :ui="{ base: 'resize-y' }"
    />
    <div v-else class="min-h-32 max-h-72 overflow-y-auto p-3">
      <WorkflowMarketDocument v-if="value.trim()" :content="value" />
      <p v-else class="text-sm text-muted">{{ t('workflow.markdown.empty') }}</p>
    </div>
    <div
      class="flex items-center justify-between gap-3 border-t border-default px-3 py-1.5 text-xs text-muted"
    >
      <span>{{ t('workflow.markdown.hint') }}</span
      ><span :class="{ 'text-error': characters > maxChars }"
        >{{ characters }} / {{ maxChars }}</span
      >
    </div>
    <p v-if="characters > maxChars" role="alert" class="px-3 pb-2 text-xs text-error">
      {{ t('workflow.markdown.too_long') }}
    </p>
  </div>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EditorToolbarItem } from '@nuxt/ui'
import WorkflowMarketDocument from './WorkflowMarketDocument.vue'
const props = withDefaults(
  defineProps<{
    modelValue: string
    label: string
    placeholder?: string
    disabled?: boolean
    maxChars?: number
  }>(),
  { maxChars: 32768 },
)
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const value = computed({
  get: () => props.modelValue,
  set: (text: string) => emit('update:modelValue', text),
})
const { t } = useI18n()
const modes = ['edit', 'source', 'preview'] as const
const mode = ref<(typeof modes)[number]>('edit')
const characters = computed(() => Array.from(value.value).length)
const toolbar = computed<EditorToolbarItem[][]>(() => [
  [
    {
      kind: 'mark',
      mark: 'bold',
      icon: 'i-tabler-bold',
      'aria-label': t('workflow.markdown.bold'),
      tooltip: { text: t('workflow.markdown.bold') },
    },
    {
      kind: 'mark',
      mark: 'italic',
      icon: 'i-tabler-italic',
      'aria-label': t('workflow.markdown.italic'),
      tooltip: { text: t('workflow.markdown.italic') },
    },
    {
      kind: 'heading',
      level: 2,
      icon: 'i-tabler-h-2',
      'aria-label': t('workflow.markdown.heading'),
      tooltip: { text: t('workflow.markdown.heading') },
    },
  ],
  [
    {
      kind: 'bulletList',
      icon: 'i-tabler-list',
      'aria-label': t('workflow.markdown.list'),
      tooltip: { text: t('workflow.markdown.list') },
    },
    {
      kind: 'orderedList',
      icon: 'i-tabler-list-numbers',
      'aria-label': t('workflow.markdown.ordered'),
      tooltip: { text: t('workflow.markdown.ordered') },
    },
    {
      kind: 'blockquote',
      icon: 'i-tabler-blockquote',
      'aria-label': t('workflow.markdown.quote'),
      tooltip: { text: t('workflow.markdown.quote') },
    },
    {
      kind: 'codeBlock',
      icon: 'i-tabler-code',
      'aria-label': t('workflow.markdown.code'),
      tooltip: { text: t('workflow.markdown.code') },
    },
  ],
  [
    {
      kind: 'undo',
      icon: 'i-tabler-arrow-back-up',
      'aria-label': t('workflow.markdown.undo'),
      tooltip: { text: t('workflow.markdown.undo') },
    },
    {
      kind: 'redo',
      icon: 'i-tabler-arrow-forward-up',
      'aria-label': t('workflow.markdown.redo'),
      tooltip: { text: t('workflow.markdown.redo') },
    },
  ],
])
</script>

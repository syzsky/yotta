<template>
  <div class="grid grid-cols-1 gap-5 sm:grid-cols-2">
    <template v-for="component in components" :key="component.id">
      <section
        v-if="component.kind === 'group'"
        class="min-w-0 space-y-4 border-t border-default pt-5 sm:col-span-2"
      >
        <h2 class="flex items-center gap-2 text-sm font-medium text-highlighted">
          <UIcon
            v-if="component.icon"
            :name="component.icon"
            class="size-4 shrink-0"
            aria-hidden="true"
          />{{ t(component.titleKey) }}
        </h2>
        <PanelRenderer
          :components="component.children ?? []"
          :snapshot="snapshot"
          :disabled="disabled"
          :busy="busy"
          :now="now"
          :waiting-buttons="waitingButtons"
          @action="(c, value) => emit('action', c, value)"
        />
      </section>
      <PanelControl
        v-else-if="['select', 'toggle', 'input'].includes(component.kind)"
        :component="component"
        :value="value(component)"
        :disabled="disabled || !!busy"
        :pending="busy === component.id"
        @change="emit('action', component, $event)"
      />
      <div v-else-if="component.kind === 'button'" class="flex items-end">
        <UButton
          color="neutral"
          variant="soft"
          :icon="component.icon"
          :loading="busy === component.id"
          :disabled="
            disabled ||
            !!busy ||
            (waitingButtons !== undefined &&
              !waitingButtons.includes(component.id) &&
              !waitingButtons.includes(''))
          "
          @click="emit('action', component, null)"
          >{{ t(component.titleKey) }}</UButton
        >
      </div>
      <PanelLog
        v-else-if="component.kind === 'log'"
        class="sm:col-span-2"
        :title="t(component.titleKey)"
        :icon="component.icon"
        :records="snapshot?.records?.[component.id] ?? []"
      />
      <div v-else class="min-w-0 space-y-1.5">
        <p class="flex items-center gap-2 text-xs text-muted">
          <UIcon
            v-if="component.icon"
            :name="component.icon"
            class="size-4 shrink-0"
            aria-hidden="true"
          />{{ t(component.titleKey) }}
        </p>
        <div
          v-if="component.kind === 'status'"
          class="flex items-center gap-2 text-sm"
          :class="!disabled && value(component) ? 'text-primary' : 'text-muted'"
        >
          <UIcon
            :name="!disabled && value(component) ? 'i-tabler-circle-check' : 'i-tabler-clock-pause'"
            class="size-4"
          />
          {{ t(!disabled && value(component) ? 'panels.valid' : 'panels.not_valid') }}
        </div>
        <UProgress
          v-else-if="component.kind === 'progress'"
          :model-value="Number(value(component) ?? 0)"
        />
        <p v-else class="break-words text-base font-medium tabular-nums text-highlighted">
          {{ formatted(component)
          }}<span v-if="component.unitKey" class="ml-1 text-xs font-normal text-muted">{{
            t(component.unitKey)
          }}</span>
        </p>
      </div>
    </template>
  </div>
</template>
<script setup lang="ts">
import { watch } from 'vue'
import { ensureWorkflowIcons } from '@/lib/workflowIcons'
import { useI18n } from 'vue-i18n'
import type { PanelComponent, PanelSnapshot } from '@/lib/panels'
import PanelControl from './PanelControl.vue'
import PanelLog from './PanelLog.vue'
const props = defineProps<{
  waitingButtons?: string[]
  components: PanelComponent[]
  snapshot: PanelSnapshot | null
  disabled: boolean
  busy: string
  now: number
}>()
const emit = defineEmits<{ action: [component: PanelComponent, value: unknown] }>()
const { t, locale } = useI18n()
watch(
  () => props.components.map((c) => c.icon).filter((icon): icon is string => Boolean(icon)),
  (icons) => {
    void ensureWorkflowIcons(icons).catch(() => undefined)
  },
  { immediate: true },
)
function value(c: PanelComponent) {
  return c.field ? props.snapshot?.values[c.field] : undefined
}
function formatted(c: PanelComponent) {
  const v = value(c)
  if (v === undefined) return '—'
  if (c.kind === 'timer') {
    const seconds = Math.max(0, Math.floor((props.now - Number(v)) / 1000))
    return `${Math.floor(seconds / 3600)
      .toString()
      .padStart(2, '0')}:${Math.floor((seconds / 60) % 60)
      .toString()
      .padStart(2, '0')}:${(seconds % 60).toString().padStart(2, '0')}`
  }
  return typeof v === 'number'
    ? new Intl.NumberFormat(locale.value, { maximumFractionDigits: c.precision ?? 2 }).format(v)
    : String(v)
}
</script>

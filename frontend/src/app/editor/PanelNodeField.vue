<template>
  <div class="space-y-2" :data-testid="component ? 'panel-component-field' : 'panel-target-field'">
    <UFormField :label="t(component ? 'panels.select_component' : 'panels.select_panel')">
      <USelectMenu
        v-if="component || override || connected"
        :model-value="value || undefined"
        :items="displayOptions"
        :aria-label="t(component ? 'panels.select_component' : 'panels.select_panel')"
        value-key="value"
        label-key="label"
        :placeholder="t(component ? 'panels.select_component' : 'panels.follow_default')"
        class="w-full"
        :disabled="connected"
        @update:model-value="emit('change', String($event))"
      />
      <div v-else class="flex items-center gap-2 py-1 text-xs">
        <UBadge size="xs" color="neutral" variant="subtle">{{ t('panels.follow_default') }}</UBadge>
        <span class="truncate text-toned">{{
          options.find((item) => item.value === value)?.label || t('panels.default_unset')
        }}</span>
      </div>
    </UFormField>
    <UCheckbox
      v-if="!component && !connected"
      :model-value="override"
      :label="t('panels.override_panel')"
      @update:model-value="$event === true ? emit('change', value || panel) : emit('inherit')"
    />
    <PanelIdentity
      v-if="value && !connected"
      :id="value"
      :label="t(component ? 'panels.component_id' : 'panels.panel_id')"
    />
    <p v-if="connected" class="text-xs text-muted">{{ t('panels.from_connection') }}</p>
    <template v-else>
      <p v-if="component && !options.length" class="text-xs text-warning">
        {{ t('panels.no_matching_component') }}
      </p>
      <RouterLink to="/panels" class="text-xs text-primary hover:underline">{{
        t('panels.manage')
      }}</RouterLink>
    </template>
    <p v-if="failure" role="alert" class="text-xs text-error">{{ failure }}</p>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePanelCatalog } from '@/composables/usePanelCatalog'
import PanelIdentity from '@/components/panels/PanelIdentity.vue'
import type { PanelComponent } from '@/lib/panels'
const props = defineProps<{
  component: boolean
  kind?: string
  panel: string
  value: string
  override: boolean
  connected?: boolean
  write?: boolean
}>()
const emit = defineEmits<{ change: [string]; inherit: [] }>()
const { t, te } = useI18n()
const { items, failure, refresh } = usePanelCatalog()
const label = (s: string) => (te(s) ? t(s) : s)
function flatten(items: PanelComponent[]): PanelComponent[] {
  return items.flatMap((c) => [c, ...flatten(c.children ?? [])])
}
const options = computed(() => {
  if (!props.component)
    return items.value.map((p) => ({
      value: p.id,
      label:
        label(p.definition.titleKey) +
        (items.value.filter((x) => x.definition.titleKey === p.definition.titleKey).length > 1
          ? ' · ' + p.id.slice(-6)
          : ''),
    }))
  const p = items.value.find((p) => p.id === props.panel)
  if (!p) return []
  const candidates = flatten(p.definition.components)
    .filter((c) => !props.write || p.managed || Boolean(c.event))
    .filter((c) =>
      props.kind === 'event'
        ? Boolean(c.event)
        : props.kind === 'log'
          ? c.kind === 'log'
          : p.definition.fields?.some((f) => f.id === c.field && f.kind === props.kind),
    )
  return candidates.map((c) => ({
    value: c.id,
    label:
      label(c.titleKey) +
      (candidates.filter((x) => x.titleKey === c.titleKey).length > 1
        ? ' · ' + c.id.slice(-6)
        : ''),
  }))
})
const displayOptions = computed(() =>
  props.value && !options.value.some((o) => o.value === props.value)
    ? [{ value: props.value, label: t('panels.missing_selection') }, ...options.value]
    : options.value,
)
onMounted(refresh)
</script>

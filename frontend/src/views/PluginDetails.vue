<template>
  <section class="space-y-4" data-testid="plugin-details-content">
    <div class="flex flex-wrap items-center justify-between gap-2 border-b border-default pb-3">
      <div class="flex flex-wrap gap-1" role="tablist" :aria-label="t('settingsPlugins.details')">
        <UButton
          v-for="option in tabs"
          :key="option.key"
          size="sm"
          :color="tab === option.key ? 'primary' : 'neutral'"
          :variant="tab === option.key ? 'soft' : 'ghost'"
          role="tab"
          :id="`${uid}-${option.key}`"
          :aria-controls="`${uid}-panel`"
          :tabindex="tab === option.key ? 0 : -1"
          :aria-selected="tab === option.key"
          @keydown="onTabKey($event)"
          @click="tab = option.key"
          >{{ t(option.label)
          }}<span v-if="option.key === 'references'" class="ml-1 tabular-nums">{{
            item.workflows.length
          }}</span></UButton
        >
      </div>
      <UButton
        v-if="item.canRollback"
        size="xs"
        color="neutral"
        variant="soft"
        :disabled="busy || inUse"
        @click="emit('rollback')"
        >{{ t('settingsPlugins.rollback') }}</UButton
      >
    </div>
    <div :id="`${uid}-panel`" role="tabpanel" :aria-labelledby="`${uid}-${tab}`">
      <div v-if="tab === 'overview'" class="grid gap-5 lg:grid-cols-2">
        <div class="space-y-3">
          <h4 class="text-xs font-medium text-highlighted">{{ t('settingsPlugins.nodes') }}</h4>
          <ul class="space-y-2">
            <li
              v-for="key in item.nodes"
              :key="key"
              class="flex items-center gap-2 text-xs text-muted"
            >
              <UIcon name="i-tabler-puzzle" class="size-4 shrink-0 text-primary" />{{
                te(key) ? t(key) : key
              }}
            </li>
          </ul>
        </div>
        <div class="space-y-3">
          <h4 class="text-xs font-medium text-highlighted">{{ t('settingsPlugins.services') }}</h4>
          <p v-if="!item.companions.length" class="text-xs text-muted">
            {{ t('settingsPlugins.services_empty') }}
          </p>
          <div
            v-for="companion in item.companions"
            :key="companion.id"
            class="flex items-center gap-3"
          >
            <UIcon name="i-tabler-activity" class="size-4 shrink-0 text-muted" />
            <div class="min-w-0 flex-1">
              <p class="text-xs text-highlighted">{{ companion.name }}</p>
              <p class="mt-1 text-xs" :class="companion.running ? 'text-primary' : 'text-muted'">
                {{ t(companion.running ? 'settingsPlugins.running' : 'settingsPlugins.stopped') }}
              </p>
            </div>
            <UButton
              size="xs"
              variant="soft"
              color="neutral"
              :disabled="busy || !item.enabled || inUse"
              @click="emit('control', companion.id, !companion.running)"
              >{{
                t(companion.running ? 'settingsPlugins.stop' : 'settingsPlugins.start')
              }}</UButton
            >
          </div>
          <p v-if="!item.enabled && item.companions.length" class="text-xs text-muted">
            {{ t('settingsPlugins.disabled_service_hint') }}
          </p>
        </div>
        <p v-if="inUse" class="text-xs text-warning lg:col-span-2">
          {{ t('settingsPlugins.in_use') }}
        </p>
      </div>
      <PluginReferences v-else-if="tab === 'references'" :workflows="item.workflows" />
      <PluginStorage v-else :plugin-id="item.id" />
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed, ref, useId, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PluginView } from '@/lib/plugins'
import PluginReferences from './PluginReferences.vue'
import PluginStorage from './PluginStorage.vue'
const props = defineProps<{ item: PluginView; busy: boolean }>()
const emit = defineEmits<{ control: [id: string, start: boolean]; rollback: [] }>()
const { t, te } = useI18n()
const uid = useId()
const tab = ref<'overview' | 'references' | 'storage'>('overview')
const tabs = [
  { key: 'overview', label: 'settingsPlugins.overview' },
  { key: 'references', label: 'settingsPlugins.references' },
  { key: 'storage', label: 'settingsPlugins.storage' },
] as const
const inUse = computed(
  () => props.item.inUse || props.item.workflows.some((workflow) => workflow.running),
)
function onTabKey(event: KeyboardEvent) {
  let index = tabs.findIndex((option) => option.key === tab.value)
  if (event.key === 'ArrowRight') index = (index + 1) % tabs.length
  else if (event.key === 'ArrowLeft') index = (index + tabs.length - 1) % tabs.length
  else if (event.key === 'Home') index = 0
  else if (event.key === 'End') index = tabs.length - 1
  else return
  event.preventDefault()
  tab.value = tabs[index]!.key
  void nextTick(() => document.getElementById(`${uid}-${tab.value}`)?.focus())
}
</script>

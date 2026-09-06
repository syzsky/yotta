<template>
  <section class="space-y-3" data-testid="plugin-references">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <UInput
        v-model="query"
        icon="i-tabler-search"
        class="w-full sm:w-72"
        :placeholder="t('settingsPlugins.references_search')"
        :aria-label="t('settingsPlugins.references_search')"
      />
      <UCheckbox v-model="onlyRunning" :label="t('settingsPlugins.only_running')" />
    </div>
    <p class="text-xs text-muted" role="status">
      {{
        t('settingsPlugins.references_count', { count: matches.length, total: workflows.length })
      }}
    </p>
    <div
      v-if="visible.length"
      class="max-h-72 overflow-auto rounded-lg border border-default divide-y divide-default"
    >
      <RouterLink
        v-for="workflow in visible"
        :key="workflow.id"
        :to="`/workflows/${workflow.id}/edit`"
        class="flex items-center gap-3 px-3 py-3 hover:bg-elevated focus-visible:outline-2 focus-visible:outline-primary"
        data-testid="plugin-reference-row"
      >
        <UIcon name="i-tabler-route" class="size-4 shrink-0 text-muted" />
        <span class="min-w-0 flex-1 truncate text-sm text-highlighted" :title="workflow.name">{{
          workflow.name
        }}</span>
        <UBadge v-if="workflow.running" size="sm" color="primary" variant="soft">{{
          t('settingsPlugins.active_run')
        }}</UBadge>
        <UIcon name="i-tabler-arrow-up-right" class="size-4 shrink-0 text-muted" />
      </RouterLink>
    </div>
    <p v-else class="py-6 text-center text-xs text-muted">
      {{
        t(
          workflows.length
            ? 'settingsPlugins.references_no_match'
            : 'settingsPlugins.references_empty',
        )
      }}
    </p>
    <footer v-if="matches.length" class="flex flex-wrap items-center justify-between gap-2">
      <span class="text-xs text-muted">{{
        t('settingsPlugins.references_page', { page: currentPage, pages })
      }}</span>
      <div class="flex gap-2">
        <UButton
          size="xs"
          variant="soft"
          color="neutral"
          :disabled="currentPage === 1"
          @click="page = currentPage - 1"
          >{{ t('settingsPlugins.previous') }}</UButton
        >
        <UButton
          size="xs"
          variant="soft"
          color="neutral"
          :disabled="currentPage === pages"
          @click="page = currentPage + 1"
          >{{ t('settingsPlugins.next') }}</UButton
        >
      </div>
    </footer>
  </section>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PluginView } from '@/lib/plugins'
const props = defineProps<{ workflows: PluginView['workflows'] }>()
const { t } = useI18n()
const query = ref('')
const onlyRunning = ref(false)
const page = ref(1)
const matches = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase()
  return props.workflows
    .filter(
      (workflow) =>
        (!onlyRunning.value || workflow.running) &&
        (!needle || workflow.name.toLocaleLowerCase().includes(needle)),
    )
    .sort(
      (left, right) =>
        left.name.localeCompare(right.name, undefined, { numeric: true }) ||
        left.id.localeCompare(right.id),
    )
})
const pages = computed(() => Math.max(1, Math.ceil(matches.value.length / 10)))
const currentPage = computed(() => Math.min(page.value, pages.value))
const visible = computed(() =>
  matches.value.slice((currentPage.value - 1) * 10, currentPage.value * 10),
)
watch([query, onlyRunning], () => {
  page.value = 1
})
watch(pages, (value) => {
  page.value = Math.min(page.value, value)
})
</script>

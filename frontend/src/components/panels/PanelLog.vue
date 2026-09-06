<template>
  <section class="min-w-0 space-y-2">
    <div class="flex items-center justify-between gap-3">
      <h3 class="flex items-center gap-2 text-xs font-medium text-highlighted">
        <UIcon v-if="icon" :name="icon" class="size-4 shrink-0" aria-hidden="true" />{{ title }}
      </h3>
      <UButton v-if="!following" size="xs" color="neutral" variant="ghost" @click="follow">{{
        t('panels.follow')
      }}</UButton>
    </div>
    <div
      ref="scroller"
      class="max-h-48 overflow-y-auto rounded-lg bg-sunken p-3 text-xs"
      :aria-label="title"
      @scroll="onScroll"
    >
      <p v-if="!records.length" class="py-4 text-center text-muted">{{ t('panels.no_records') }}</p>
      <div v-for="record in records.slice(-200)" :key="record.id" class="flex gap-3 py-1">
        <time class="shrink-0 tabular-nums text-muted">{{
          new Date(record.timeMs).toLocaleTimeString()
        }}</time>
        <span class="min-w-0 break-words text-default">{{
          record.messageKey ? t(record.messageKey) : record.text
        }}</span>
      </div>
    </div>
    <p v-if="records.length > 200" class="text-xs text-muted">
      {{ t('panels.recent_records', { count: 200 }) }}
    </p>
  </section>
</template>
<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PanelRecord } from '@/lib/panels'
const props = defineProps<{ title: string; records: PanelRecord[]; icon?: string }>()
const { t } = useI18n()
const scroller = ref<HTMLElement>()
const following = ref(true)
function onScroll() {
  const el = scroller.value
  if (el) following.value = el.scrollHeight - el.scrollTop - el.clientHeight < 24
}
async function follow() {
  following.value = true
  await nextTick()
  if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
}
watch(
  () => props.records.at(-1)?.id,
  () => {
    if (following.value) void follow()
  },
)
</script>

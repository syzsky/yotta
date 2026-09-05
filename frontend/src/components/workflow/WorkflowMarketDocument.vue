<template>
  <div class="market-prose" @click="openLink" v-html="html" />
  <p v-if="failure" role="alert" class="mt-2 text-sm text-error">{{ failure }}</p>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { Browser } from '@wailsio/runtime'
import { errorMessage } from '@/lib/invoke'
const props = defineProps<{ content: string }>()
const html = ref(''),
  failure = ref('')
let generation = 0
watch(
  () => props.content,
  async (content) => {
    const ticket = ++generation
    const { renderMarkdown } = await import('@/lib/markdown')
    if (ticket === generation) html.value = renderMarkdown(content)
  },
  { immediate: true },
)
async function openLink(event: MouseEvent) {
  const link = (event.target as Element).closest('a')
  if (!link) return
  event.preventDefault()
  if (!/^https?:\/\//i.test(link.href)) return
  try {
    await Browser.OpenURL(link.href)
  } catch (error) {
    failure.value = errorMessage(error)
  }
}
</script>
<style scoped>
.market-prose {
  color: var(--ui-text);
  font-size: 14px;
  line-height: 1.85;
  overflow-wrap: anywhere;
}
.market-prose :deep(h1),
.market-prose :deep(h2),
.market-prose :deep(h3) {
  margin: 24px 0 12px;
  font-weight: 600;
  color: var(--ui-text-highlighted);
  line-height: 1.4;
}
.market-prose :deep(h1) {
  font-size: 24px;
}
.market-prose :deep(h2) {
  font-size: 20px;
}
.market-prose :deep(h3) {
  font-size: 16px;
}
.market-prose :deep(p) {
  margin: 0 0 12px;
}
.market-prose :deep(ul) {
  list-style: disc;
  padding-left: 22px;
}
.market-prose :deep(ol) {
  list-style: decimal;
  padding-left: 22px;
}
.market-prose :deep(a) {
  color: var(--ui-primary);
  text-decoration: underline;
}
.market-prose :deep(pre) {
  overflow-x: auto;
  padding: 12px;
  background: var(--ui-bg-muted);
  border-radius: 6px;
}
.market-prose :deep(img) {
  max-width: 100%;
  height: auto;
}
</style>

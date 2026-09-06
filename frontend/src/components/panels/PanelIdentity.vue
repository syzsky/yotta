<template>
  <details class="text-xs text-muted" data-testid="panel-identity">
    <summary
      class="w-fit cursor-pointer rounded py-1 focus-visible:outline-2 focus-visible:outline-primary"
    >
      {{ label }} · <span class="font-mono">{{ id.slice(-6) }}</span>
    </summary>
    <div class="mt-1 flex min-w-0 flex-wrap items-center gap-2">
      <code class="min-w-0 select-all break-all text-toned">{{ id }}</code>
      <UButton color="neutral" variant="ghost" size="xs" icon="i-tabler-copy" @click="copy">{{
        t(copied ? 'panels.id_copied' : 'panels.copy_id')
      }}</UButton>
    </div>
    <p v-if="copyFailed" role="status" class="mt-1">{{ t('panels.copy_id_manual') }}</p>
  </details>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ id: string; label: string }>()
const { t } = useI18n()
const copied = ref(false),
  copyFailed = ref(false)
watch(
  () => props.id,
  () => {
    copied.value = false
    copyFailed.value = false
  },
)
async function copy() {
  try {
    await navigator.clipboard.writeText(props.id)
    copied.value = true
    copyFailed.value = false
  } catch {
    copyFailed.value = true
  }
}
</script>

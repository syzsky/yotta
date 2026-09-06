<template>
  <UButton
    size="xs"
    color="neutral"
    variant="ghost"
    class="max-w-full font-mono"
    :icon="copied ? 'i-tabler-check' : 'i-tabler-copy'"
    :label="full ? id : id.slice(-6)"
    :title="id"
    :aria-label="label + ' ' + id"
    @click="copy"
  />
  <span v-if="failed" role="status" class="text-xs text-error">{{
    t('panels.copy_id_manual')
  }}</span>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
const props = defineProps<{ id: string; label: string; full?: boolean }>()
const { t } = useI18n()
const copied = ref(false),
  failed = ref(false)
watch(
  () => props.id,
  () => {
    copied.value = false
    failed.value = false
  },
)
async function copy() {
  const id = props.id
  try {
    await navigator.clipboard.writeText(id)
    if (props.id === id) {
      copied.value = true
      failed.value = false
    }
  } catch {
    if (props.id === id) failed.value = true
  }
}
</script>

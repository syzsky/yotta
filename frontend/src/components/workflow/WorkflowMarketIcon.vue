<template><UIcon :name="resolved" /></template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { resolveWorkflowIcon } from '@/lib/workflowIcons'
const props = defineProps<{ name?: string }>()
const resolved = ref('i-tabler-route')
let generation = 0
watch(
  () => props.name,
  async (name) => {
    const ticket = ++generation
    try {
      const value = await resolveWorkflowIcon(name)
      if (ticket === generation) resolved.value = value
    } catch {
      if (ticket === generation) resolved.value = 'i-tabler-route'
    }
  },
  { immediate: true },
)
</script>

<template>
  <div class="h-full min-h-0" data-testid="workflow-path-dock">
    <WorkflowResourceDock
      kind="path"
      :source="source"
      recording-phase="idle"
      @use="(selection, position, actionId) => emit('use', selection, position, actionId)"
      @open-library="router.push('/assets?type=paths')"
    />
  </div>
</template>
<script setup lang="ts">
import { useRouter } from 'vue-router'
import WorkflowResourceDock from './WorkflowResourceDock.vue'
import type { YottaWorkflowSource } from '../../../../contracts/workflow/current/workflow-source'
import type { AssetPickerSelection } from '@/stores/assets'
defineProps<{ source: YottaWorkflowSource }>()
const emit = defineEmits<{
  use: [selection: AssetPickerSelection, position?: { x: number; y: number }, actionId?: string]
}>()
const router = useRouter()
</script>

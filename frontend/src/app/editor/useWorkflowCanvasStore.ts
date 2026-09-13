import { useVueFlow } from '@vue-flow/core'
import { useId } from 'vue'

export function useWorkflowCanvasStore() {
  // KeepAlive retains several editors; each must own its nodes, handles and callbacks.
  return useVueFlow(`workflow-editor-${useId()}`)
}

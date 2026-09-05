import { ref } from 'vue'
import type { WorkflowWorkspacePanel } from './workspacePanel'

export function useEditorPanelLayout(initialInspectorOpen: boolean) {
  const workspacePanel = ref<WorkflowWorkspacePanel>('graphs')
  const workspaceSidebarOpen = ref(true)
  const workspaceSidebarWidth = ref(320)
  const inspectorSidebarOpen = ref(initialInspectorOpen)
  const inspectorSidebarWidth = ref(360)
  let stopActiveResize: (() => void) | undefined

  function toggleWorkspacePanel(panel: WorkflowWorkspacePanel): void {
    if (workspaceSidebarOpen.value && workspacePanel.value === panel) {
      workspaceSidebarOpen.value = false
      return
    }
    workspacePanel.value = panel
    workspaceSidebarOpen.value = true
    if (workspaceSidebarWidth.value < 280) workspaceSidebarWidth.value = 320
  }

  function resizeWorkspaceSidebar(startWidth: number, deltaX: number): number {
    return Math.min(480, Math.max(240, startWidth + deltaX))
  }

  function resizeInspectorSidebar(startWidth: number, deltaX: number): number {
    return Math.min(560, Math.max(280, startWidth + deltaX))
  }

  function startSidebarResize(side: 'workspace' | 'inspector', event: PointerEvent): void {
    event.preventDefault()
    stopSidebarResize()
    const startX = event.clientX
    const startWidth =
      side === 'workspace' ? workspaceSidebarWidth.value : inspectorSidebarWidth.value
    const move = (moveEvent: PointerEvent) => {
      if (side === 'workspace') {
        workspaceSidebarWidth.value = resizeWorkspaceSidebar(startWidth, moveEvent.clientX - startX)
      } else {
        inspectorSidebarWidth.value = resizeInspectorSidebar(startWidth, startX - moveEvent.clientX)
      }
    }
    const stop = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
      if (stopActiveResize === stop) stopActiveResize = undefined
    }
    stopActiveResize = stop
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop, { once: true })
  }

  function stopSidebarResize(): void {
    stopActiveResize?.()
  }

  return {
    workspacePanel,
    workspaceSidebarOpen,
    workspaceSidebarWidth,
    inspectorSidebarOpen,
    inspectorSidebarWidth,
    toggleWorkspacePanel,
    resizeWorkspaceSidebar,
    resizeInspectorSidebar,
    startSidebarResize,
    stopSidebarResize,
  }
}
